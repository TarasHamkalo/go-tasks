package messaging

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestRepository(t *testing.T) (*SqliteRepository, context.Context) {
	t.Helper()

	tempDir := filepath.Join(".", "temp")
	err := os.MkdirAll(tempDir, 0o755)
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, t.Name()+".db")
	_ = os.Remove(dbPath)

	repo, err := NewSqliteRepository(dbPath)
	if err != nil {
		t.Fatalf("could not create repository: %v", err)
	}

	ctx := context.Background()

	err = repo.InitializeSchema(ctx)
	if err != nil {
		t.Fatalf("could not initialize schema: %v", err)
	}

	t.Cleanup(func() {
		_ = repo.Close()
		// _ = os.Remove(dbPath)
		_ = os.RemoveAll(tempDir)
	})

	return repo, ctx
}

func TestInsertChatAndGetUserChats(t *testing.T) {
	repo, ctx := newTestRepository(t)

	chat := Chat{
		Id:      "chat-1",
		Name:    "Test Chat",
		IsGroup: false,
	}

	err := repo.InsertChat(ctx, chat)
	if err != nil {
		t.Fatalf("InsertChat failed: %v", err)
	}

	err = repo.AddChatMember(ctx, chat.Id, "user-1")
	if err != nil {
		t.Fatalf("AddChatMember failed: %v", err)
	}

	chats, err := repo.GetUserChats(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetUserChats failed: %v", err)
	}

	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}

	if chats[0].Id != chat.Id {
		t.Fatalf("expected chat ID %q, got %q", chat.Id, chats[0].Id)
	}

	if chats[0].Name != chat.Name {
		t.Fatalf("expected chat name %q, got %q", chat.Name, chats[0].Name)
	}
}

func TestAddAndRemoveChatMember(t *testing.T) {
	repo, ctx := newTestRepository(t)

	chat := Chat{
		Id:      "chat-1",
		Name:    "Test Chat",
		IsGroup: true,
	}

	err := repo.InsertChat(ctx, chat)
	if err != nil {
		t.Fatalf("InsertChat failed: %v", err)
	}

	err = repo.AddChatMember(ctx, chat.Id, "user-1")
	if err != nil {
		t.Fatalf("AddChatMember failed: %v", err)
	}

	err = repo.AddChatMember(ctx, chat.Id, "user-2")
	if err != nil {
		t.Fatalf("AddChatMember failed: %v", err)
	}

	members, err := repo.GetChatMembers(ctx, chat.Id)
	if err != nil {
		t.Fatalf("GetChatMembers failed: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	err = repo.RemoveChatMember(ctx, chat.Id, "user-2")
	if err != nil {
		t.Fatalf("RemoveChatMember failed: %v", err)
	}

	members, err = repo.GetChatMembers(ctx, chat.Id)
	if err != nil {
		t.Fatalf("GetChatMembers failed: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	if members[0] != "user-1" {
		t.Fatalf("expected remaining member user-1, got %q", members[0])
	}
}

func TestInsertMessageWithAcksAndGetUndeliveredMessages(t *testing.T) {
	repo, ctx := newTestRepository(t)

	chat := Chat{
		Id:      "chat-1",
		Name:    "Direct Chat",
		IsGroup: false,
	}

	err := repo.InsertChat(ctx, chat)
	if err != nil {
		t.Fatalf("InsertChat failed: %v", err)
	}

	message := Message{
		Id:       "msg-1",
		ChatId:   chat.Id,
		SenderId: "user-1",
		Content:  []byte("hello"),
		SentAt:   time.Now().UTC(),
	}

	acks := []MessageAck{
		{
			MessageId: message.Id,
			UserId:    "user-2",
		},
		{
			MessageId: message.Id,
			UserId:    "user-3",
		},
	}

	err = repo.InsertMessageWithAcks(ctx, message, acks)
	if err != nil {
		t.Fatalf("InsertMessageWithAcks failed: %v", err)
	}

	messages, err := repo.GetUndeliveredMessages(ctx, "user-2")
	if err != nil {
		t.Fatalf("GetUndeliveredMessages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 undelivered message, got %d", len(messages))
	}

	if messages[0].Id != message.Id {
		t.Fatalf("expected message ID %q, got %q", message.Id, messages[0].Id)
	}

	if string(messages[0].Content) != "hello" {
		t.Fatalf(
			"expected message content %q, got %q",
			"hello",
			string(messages[0].Content),
		)
	}
}

func TestSetMessageDelivered(t *testing.T) {
	repo, ctx := newTestRepository(t)

	chat := Chat{
		Id:      "chat-1",
		Name:    "Direct Chat",
		IsGroup: false,
	}

	err := repo.InsertChat(ctx, chat)
	if err != nil {
		t.Fatalf("InsertChat failed: %v", err)
	}

	message := Message{
		Id:       "msg-1",
		ChatId:   chat.Id,
		SenderId: "user-1",
		Content:  []byte("hello"),
		SentAt:   time.Now().UTC(),
	}

	acks := []MessageAck{
		{
			MessageId: message.Id,
			UserId:    "user-2",
		},
	}

	err = repo.InsertMessageWithAcks(ctx, message, acks)
	if err != nil {
		t.Fatalf("InsertMessageWithAcks failed: %v", err)
	}

	deliveredAt := time.Now().UTC()

	err = repo.SetMessageDelivered(
		ctx,
		message.Id,
		"user-2",
		deliveredAt,
	)
	if err != nil {
		t.Fatalf("SetMessageDelivered failed: %v", err)
	}

	storedAcks, err := repo.GetMessageAcks(ctx, message.Id)
	if err != nil {
		t.Fatalf("GetMessageAcks failed: %v", err)
	}

	if len(storedAcks) != 1 {
		t.Fatalf("expected 1 ack, got %d", len(storedAcks))
	}

	if !storedAcks[0].DeliveredAt.Valid {
		t.Fatal("expected DeliveredAt to be set")
	}
}

func TestSetMessageRead(t *testing.T) {
	repo, ctx := newTestRepository(t)

	chat := Chat{
		Id:      "chat-1",
		Name:    "Direct Chat",
		IsGroup: false,
	}

	err := repo.InsertChat(ctx, chat)
	if err != nil {
		t.Fatalf("InsertChat failed: %v", err)
	}

	message := Message{
		Id:       "msg-1",
		ChatId:   chat.Id,
		SenderId: "user-1",
		Content:  []byte("hello"),
		SentAt:   time.Now().UTC(),
	}

	acks := []MessageAck{
		{
			MessageId: message.Id,
			UserId:    "user-2",
		},
	}

	err = repo.InsertMessageWithAcks(ctx, message, acks)
	if err != nil {
		t.Fatalf("InsertMessageWithAcks failed: %v", err)
	}

	readAt := time.Now().UTC()

	err = repo.SetMessageRead(
		ctx,
		message.Id,
		"user-2",
		readAt,
	)
	if err != nil {
		t.Fatalf("SetMessageRead failed: %v", err)
	}

	storedAcks, err := repo.GetMessageAcks(ctx, message.Id)
	if err != nil {
		t.Fatalf("GetMessageAcks failed: %v", err)
	}

	if len(storedAcks) != 1 {
		t.Fatalf("expected 1 ack, got %d", len(storedAcks))
	}

	if !storedAcks[0].ReadAt.Valid {
		t.Fatal("expected ReadAt to be set")
	}
}

func TestInsertMessageWithAcksRollbackOnInvalidAck(t *testing.T) {
	repo, ctx := newTestRepository(t)

	chat := Chat{
		Id:      "chat-1",
		Name:    "Direct Chat",
		IsGroup: false,
	}

	err := repo.InsertChat(ctx, chat)
	if err != nil {
		t.Fatalf("InsertChat failed: %v", err)
	}

	message := Message{
		Id:       "msg-1",
		ChatId:   chat.Id,
		SenderId: "user-1",
		Content:  []byte("hello"),
		SentAt:   time.Now().UTC(),
	}

	// Duplicate (message_id, user_id) violates primary key.
	acks := []MessageAck{
		{
			MessageId: message.Id,
			UserId:    "user-2",
		},
		{
			MessageId: message.Id,
			UserId:    "user-2",
		},
	}

	err = repo.InsertMessageWithAcks(ctx, message, acks)
	if err == nil {
		t.Fatal("expected InsertMessageWithAcks to fail")
	}

	messages, err := repo.GetUndeliveredMessages(ctx, "user-2")
	if err != nil {
		t.Fatalf("GetUndeliveredMessages failed: %v", err)
	}

	if len(messages) != 0 {
		t.Fatalf(
			"expected rollback to remove inserted message, got %d messages",
			len(messages),
		)
	}
}

// Prevent unused import warning if sql.NullTime changes in future tests.
var _ = sql.NullTime{}

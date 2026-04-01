package core

import (
	"context"
	"fmt"
	"sync"
)

type TaskEntry struct {
	Task   *DownloadTask
	Cancel context.CancelFunc
}

func NewTaskEntry(task *DownloadTask, cancel context.CancelFunc) *TaskEntry {
	return &TaskEntry{Task: task, Cancel: cancel}
}

type TasksStore struct {
	entries map[string]*TaskEntry
	lock    sync.RWMutex
}

func NewTasksStore() *TasksStore {
	return &TasksStore{
		entries: make(map[string]*TaskEntry, 10),
		lock:    sync.RWMutex{},
	}
}

func (s *TasksStore) Remove(taskId string) (*TaskEntry, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	entry, ok := s.entries[taskId]
	if ok {
		delete(s.entries, taskId)
		return entry, nil
	}

	return nil, fmt.Errorf("task %s not found", taskId)
}

func (s *TasksStore) Get(id string) (*TaskEntry, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	entry, ok := s.entries[id]
	if ok {
		return entry, nil
	}

	return nil, fmt.Errorf("task %s not found", id)
}

func (s *TasksStore) Add(entry *TaskEntry) {
	// TODO: here should check whether id is not taken
	s.lock.Lock()
	defer s.lock.Unlock()

	s.entries[entry.Task.Id()] = entry
}

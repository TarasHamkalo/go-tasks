package state

// Profile stores profile data fetched from the server
type Profile struct {
	Id       string
	Username string
}

// Chat interface abstracts the display logic from the TUI renderer
type Chat interface {
	Id() string
	IsGroup() bool
	Name() string
}

// DirectChat resolves its name based on the other person's profile
type DirectChat struct {
	ChatId       string
	OtherProfile *Profile
}

func (c *DirectChat) Id() string {
	return c.ChatId
}

func (c *DirectChat) IsGroup() bool {
	return false
}

func (c *DirectChat) Name() string {
	if c.OtherProfile != nil && c.OtherProfile.Username != "" {
		return c.OtherProfile.Username
	}
	return "Unknown User"
}

// GroupChat resolves its name based on the server-provided group name
type GroupChat struct {
	ChatId    string
	GroupName string
}

func (c *GroupChat) Id() string {
	return c.ChatId
}

func (c *GroupChat) IsGroup() bool {
	return true
}

func (c *GroupChat) Name() string {
	return c.GroupName
}

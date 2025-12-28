package models

import "time"

type Profile struct {
	ID                  int64
	LinkedInURL         string
	Name                string
	Headline            string
	Company             string
	Location            string
	ConnectionSent      bool
	ConnectionSentAt    *time.Time
	ConnectionAccepted  bool
	ConnectionCheckedAt *time.Time
	MessageSent         bool
	MessageSentAt       *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MessageType string

const (
	MessageTypeConnectionNote MessageType = "connection_note"
	MessageTypeFollowUp       MessageType = "follow_up"
)

type MessageLog struct {
	ID        int64
	ProfileID int64
	Type      MessageType
	Content   string
	CreatedAt time.Time
}

type RunLog struct {
	ID        int64
	RunType   string
	StartedAt time.Time
	EndedAt   time.Time
	Summary   string
}

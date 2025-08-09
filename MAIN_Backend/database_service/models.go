package database_service

import (
	"time"
)

// Status represents the application status lifecycle.
type Status string

const (
	StatusApplicant        Status = "applicant"
	StatusInterviewPending Status = "interview_pending"
	StatusMember           Status = "member"
	StatusBanned           Status = "banned"
)

// User is a projection of the `users` table.
type User struct {
	ID              string    `json:"id"`
	DiscordUserID   int64     `json:"discord_user_id"`
	DiscordUsername string    `json:"discord_username"`
	MinecraftName   *string   `json:"minecraft_name,omitempty"`
	Age             *int16    `json:"age,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Application mirrors the `applications` table.
type Application struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Answers   map[string]any `json:"answers"`
	Status    Status         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// LoginToken represents a temporary token linked to a user for web login
type LoginToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	AddedAt   time.Time `json:"added_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

// AppEvent is emitted by triggers via LISTEN/NOTIFY on channel `app_events`.
// Only a subset of fields may be present depending on the table/action.
type AppEvent struct {
	Table         string    `json:"table"`
	Action        string    `json:"action"`
	RowID         string    `json:"row_id"`
	UserID        *string   `json:"user_id,omitempty"`
	Status        *Status   `json:"status,omitempty"`
	MinecraftName *string   `json:"minecraft_name,omitempty"`
	DiscordUserID *int64    `json:"discord_user_id,omitempty"`
	At            time.Time `json:"at"`
}

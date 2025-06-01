package database

import (
	"time"
)

// AuditLog represents system audit log entries (legacy format)
type AuditLog struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64     `gorm:"index" json:"user_id"`
	Action      string    `gorm:"size:50;not null" json:"action"`
	EntityType  string    `gorm:"size:50;not null" json:"entity_type"`
	EntityID    int64     `gorm:"not null" json:"entity_id"`
	Description string    `gorm:"type:text" json:"description"`
	IP          string    `gorm:"size:50" json:"ip"`
	UserAgent   string    `gorm:"size:255" json:"user_agent"`
	CreatedAt   time.Time `json:"created_at"`
}

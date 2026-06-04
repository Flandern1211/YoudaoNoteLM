package entity

import "time"

// AudioPreview 音频预览实体
type AudioPreview struct {
	BaseEntity
	PreviewID       string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_preview_id" json:"preview_id"`
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	NotebookID      uint      `gorm:"not null" json:"notebook_id"`
	FileName        string    `gorm:"type:varchar(255);not null" json:"file_name"`
	FilePath        string    `gorm:"type:varchar(512);not null" json:"file_path"`
	FileSize        int64     `json:"file_size"`
	TranscribedText string    `gorm:"type:longtext;not null" json:"transcribed_text"`
	Status          string    `gorm:"type:varchar(20);default:pending" json:"status"`
	ExpiresAt       time.Time `gorm:"not null;index:idx_expires" json:"expires_at"`
}

func (AudioPreview) TableName() string {
	return "audio_preview"
}

package entity

// Source 资料来源实体
type Source struct {
	BaseEntity
	UserID          uint   `gorm:"not null;index:idx_user_notebook" json:"user_id"`
	NotebookID      uint   `gorm:"not null;index:idx_user_notebook" json:"notebook_id"`
	Name            string `gorm:"type:varchar(255);not null" json:"name"`
	Type            string `gorm:"type:varchar(20);not null;index:idx_type" json:"type"`
	OriginalURL     string `gorm:"type:varchar(2048)" json:"original_url"`
	FilePath        string `gorm:"type:varchar(512)" json:"file_path"`
	FileSize        int64  `json:"file_size"`
	MimeType        string `gorm:"type:varchar(100)" json:"mime_type"`
	MarkdownContent string `gorm:"type:longtext" json:"markdown_content"`
	RawContent      string `gorm:"type:longtext" json:"raw_content"`
	Status          string `gorm:"type:varchar(20);default:pending;index:idx_status" json:"status"`
	ErrorMessage    string `gorm:"type:varchar(512)" json:"error_message"`
	IsSource        bool   `gorm:"default:false" json:"is_source"`
	Vectorized      bool   `gorm:"default:false" json:"vectorized"`
}

func (Source) TableName() string {
	return "source"
}

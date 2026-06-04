package entity

// ImportTask 导入任务实体
type ImportTask struct {
	BaseEntity
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	NotebookID   uint   `gorm:"not null" json:"notebook_id"`
	TaskType     string `gorm:"type:varchar(20);not null" json:"task_type"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	FailCount    int    `json:"fail_count"`
	Status       string `gorm:"type:varchar(20);default:pending" json:"status"`
	ErrorDetail  string `gorm:"type:json" json:"error_detail"`
}

func (ImportTask) TableName() string {
	return "import_task"
}

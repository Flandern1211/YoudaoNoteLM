package entity

import (
	"time"

	"gorm.io/gorm"
)

// BaseEntity 基础实体
type BaseEntity struct {
	ID        uint           `gorm:"primarykey" json:"id"`        // 主键ID
	CreatedAt time.Time      `json:"created_at"`                  // 创建时间
	UpdatedAt time.Time      `json:"updated_at"`                  // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`              // 软删除时间
}

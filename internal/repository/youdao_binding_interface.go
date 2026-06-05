package repository

import "YoudaoNoteLm/internal/model/entity"

type YoudaoBindingRepository interface {
	FindByUserID(userID int) (*entity.YoudaoBinding, error)
	Create(binding *entity.YoudaoBinding) error
	Update(binding *entity.YoudaoBinding) error
	Delete(userID int) error
}

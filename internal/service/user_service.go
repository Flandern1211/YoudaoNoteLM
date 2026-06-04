package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/response"
	"context"

	bizerrors "YoudaoNoteLm/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// userService 用户服务实现
type userService struct {
	userRepo      repository.UserRepository
	verifyCodeSvc VerifyCodeService
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository, verifyCodeSvc VerifyCodeService) UserService {
	return &userService{
		userRepo:      userRepo,
		verifyCodeSvc: verifyCodeSvc,
	}
}

// Register 用户注册（邮箱+验证码）
func (s *userService) Register(ctx context.Context, req *request.RegisterRequest) error {
	// 校验验证码
	if err := s.verifyCodeSvc.Verify(ctx, req.Email, "register", req.Code); err != nil {
		return err
	}

	// 检查邮箱是否已被注册
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 自动生成用户名（邮箱前缀）
	username := generateUsername(req.Email)

	// 创建用户
	user := &entity.User{
		Username: username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   1, // 默认正常
	}

	if err := s.userRepo.Create(user); err != nil {
		return err
	}

	return nil
}

// generateUsername 从邮箱生成用户名
func generateUsername(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

// GetUserByID 根据 ID 获取用户
func (s *userService) GetUserByID(id uint) (*entity.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(id uint, req *request.UpdateUserRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 更新字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	return s.userRepo.Update(user)
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(id uint, req *request.ChangePasswordRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return s.userRepo.Update(user)
}

// GetUserResponse 获取用户响应
func (s *userService) GetUserResponse(user *entity.User) *dto.UserResponse {
	return &dto.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// ListUsers 分页获取用户列表
func (s *userService) ListUsers(req *request.UserListRequest) (*response.PageResponse, error) {
	// 参数标准化
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	// 计算偏移量
	offset := (page - 1) * size

	// 查询数据
	users, total, err := s.userRepo.List(offset, size)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	list := make([]*dto.UserResponse, 0, len(users))
	for _, user := range users {
		list = append(list, s.GetUserResponse(user))
	}

	return response.NewPageResponse(list, total, page, size), nil
}

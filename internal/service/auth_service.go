package service

import (
	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/repository"
	"YoudaoNoteLm/pkg/jwt"
	"context"
	"fmt"
	"time"

	bizerrors "YoudaoNoteLm/pkg/errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	maxLoginAttempts = 3                // 最大登录失败次数
	lockDuration     = 15 * time.Minute // 锁定时长 15 分钟
)

// authService 认证服务实现
type authService struct {
	userRepo       repository.UserRepository
	userService    UserService
	verifyCodeSvc  VerifyCodeService
	captchaSvc     CaptchaService
	tokenBlacklist TokenBlacklistService
}

// NewAuthService 创建认证服务
func NewAuthService(userRepo repository.UserRepository, userService UserService, verifyCodeSvc VerifyCodeService, captchaSvc CaptchaService, tokenBlacklist TokenBlacklistService) AuthService {
	return &authService{
		userRepo:       userRepo,
		userService:    userService,
		verifyCodeSvc:  verifyCodeSvc,
		captchaSvc:     captchaSvc,
		tokenBlacklist: tokenBlacklist,
	}
}

// Login 用户登录（邮箱+密码+滑块验证）
func (s *authService) Login(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error) {
	// 校验滑块验证码
	if err := s.captchaSvc.Verify(ctx, req.CaptchaID, req.CaptchaX); err != nil {
		return nil, err
	}

	// 根据邮箱查找用户
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// 检查账户是否被锁定
	if user.IsLocked() {
		return nil, bizerrors.ErrUserLocked
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// 密码错误，增加失败次数
		s.handleLoginFailure(user)
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 登录成功，重置失败次数
	if user.FailedAttempts > 0 || user.LockedUntil != nil {
		_ = s.userRepo.ResetLoginAttempts(user.ID)
	}

	// 生成双 Token
	tokenPair, err := jwt.GenerateTokenPair(user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	// 构建响应
	userResp := s.userService.GetUserResponse(user)
	return &dto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         *userResp,
	}, nil
}

// handleLoginFailure 处理登录失败
func (s *authService) handleLoginFailure(user *entity.User) {
	attempts := user.FailedAttempts + 1
	if attempts >= maxLoginAttempts {
		// 锁定账户 15 分钟
		lockUntil := time.Now().Add(lockDuration)
		_ = s.userRepo.LockUser(user.ID, lockUntil)
	} else {
		_ = s.userRepo.UpdateLoginAttempts(user.ID, attempts)
	}
}

// RefreshToken 用 refresh token 换取新的 token 对
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*dto.LoginResponse, error) {
	claims, err := jwt.ParseToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// 必须是 refresh token
	if claims.TokenType != jwt.RefreshToken {
		return nil, bizerrors.New(bizerrors.CodeInvalidToken, "请使用 refresh_token 进行刷新")
	}

	// 检查 refresh token 是否已被吊销
	revoked, err := s.tokenBlacklist.IsRevoked(ctx, claims.ID)
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, bizerrors.New(bizerrors.CodeInvalidToken, "refresh token 已失效，请重新登录")
	}

	// 检查用户是否存在且状态正常
	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// 将旧的 refresh token 加入黑名单（防止重放攻击）
	if err := s.tokenBlacklist.RevokeToken(ctx, refreshToken); err != nil {
		// 吊销失败仅记录日志，不影响刷新流程
		_ = err
	}

	// 生成新的 token 对
	tokenPair, err := jwt.GenerateTokenPair(user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	userResp := s.userService.GetUserResponse(user)
	return &dto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         *userResp,
	}, nil
}

// Logout 用户登出，将 access token 和 refresh token 加入黑名单
func (s *authService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	// 吊销 access token
	if accessToken != "" {
		if err := s.tokenBlacklist.RevokeToken(ctx, accessToken); err != nil {
			return fmt.Errorf("吊销 access token 失败: %w", err)
		}
	}

	// 吊销 refresh token
	if refreshToken != "" {
		if err := s.tokenBlacklist.RevokeToken(ctx, refreshToken); err != nil {
			return fmt.Errorf("吊销 refresh token 失败: %w", err)
		}
	}

	return nil
}

// SendCode 发送验证码
func (s *authService) SendCode(ctx context.Context, req *request.SendCodeRequest) (*dto.SendCodeResponse, error) {
	switch req.Type {
	case "register":
		// 注册验证码：检查邮箱是否已被注册
		exists, err := s.userRepo.ExistsByEmail(req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
		}
	case "reset":
		// 重置密码验证码：检查邮箱是否已注册
		exists, err := s.userRepo.ExistsByEmail(req.Email)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, bizerrors.ErrUserNotFound
		}
	}

	// 生成并发送验证码
	_, err := s.verifyCodeSvc.Generate(ctx, req.Email, req.Type)
	if err != nil {
		return nil, err
	}

	// 获取冷却时间
	remaining, _ := s.verifyCodeSvc.GetCooldownRemaining(ctx, req.Email, req.Type)

	return &dto.SendCodeResponse{
		RetryAfter: remaining,
	}, nil
}

// ResetPassword 重置密码
func (s *authService) ResetPassword(req *request.ResetPasswordRequest) error {
	// 校验验证码
	ctx := context.Background()
	if err := s.verifyCodeSvc.Verify(ctx, req.Email, "reset", req.Code); err != nil {
		return err
	}

	// 查找用户
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return err
	}
	if user == nil {
		return bizerrors.ErrUserNotFound
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新密码并重置锁定状态
	user.Password = string(hashedPassword)
	user.FailedAttempts = 0
	user.LockedUntil = nil
	return s.userRepo.Update(user)
}

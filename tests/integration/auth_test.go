package integration

import (
	"context"
	"testing"

	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	bizerrors "YoudaoNoteLm/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ======================== TC-001: 获取滑块验证码 ========================

func TestGetCaptcha_Success(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "GET", "/api/v1/auth/captcha", nil, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["captcha_id"] == nil {
		t.Error("captcha_id 不能为空")
	}
	if data["background"] == nil {
		t.Error("background 不能为空")
	}
	if data["slider"] == nil {
		t.Error("slider 不能为空")
	}
}

func TestGetCaptcha_ServiceError(t *testing.T) {
	env := NewTestEnv()
	env.Captcha.GenerateFunc = func(ctx context.Context) (*dto.CaptchaData, error) {
		return nil, bizerrors.ErrInternalServiceError
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/auth/captcha", nil, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInternalError)
}

// ======================== TC-002: 发送验证码 ========================

func TestSendCode_Success(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.SendCodeRequest{
		Email: "test@example.com",
		Type:  "register",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/send-code", body, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestSendCode_InvalidEmail(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"email": "invalid-email", "type": "register"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/send-code", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSendCode_InvalidType(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"email": "test@example.com", "type": "invalid"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/send-code", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestSendCode_TooFrequent(t *testing.T) {
	env := NewTestEnv()
	env.Auth.SendCodeFunc = func(ctx context.Context, req *request.SendCodeRequest) (*dto.SendCodeResponse, error) {
		return nil, bizerrors.ErrVerifyCodeTooFrequent
	}

	body := JSONBody(request.SendCodeRequest{
		Email: "test@example.com",
		Type:  "register",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/send-code", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeVerifyCodeTooFrequent)
}

// ======================== TC-003: 用户注册 ========================

func TestRegister_Success(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.RegisterRequest{
		Email:           "new@example.com",
		Password:        "Test123456",
		ConfirmPassword: "Test123456",
		Code:            "123456",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/register", body, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestRegister_PasswordMismatch(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.RegisterRequest{
		Email:           "new@example.com",
		Password:        "Test123456",
		ConfirmPassword: "Different123",
		Code:            "123456",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/register", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestRegister_PasswordTooShort(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.RegisterRequest{
		Email:           "new@example.com",
		Password:        "123",
		ConfirmPassword: "123",
		Code:            "123456",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/register", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestRegister_UserAlreadyExists(t *testing.T) {
	env := NewTestEnv()
	env.User.RegisterFunc = func(ctx context.Context, req *request.RegisterRequest) error {
		return bizerrors.ErrUserAlreadyExists
	}

	body := JSONBody(request.RegisterRequest{
		Email:           "existing@example.com",
		Password:        "Test123456",
		ConfirmPassword: "Test123456",
		Code:            "123456",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/register", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUserAlreadyExists)
}

func TestRegister_VerifyCodeInvalid(t *testing.T) {
	env := NewTestEnv()
	env.User.RegisterFunc = func(ctx context.Context, req *request.RegisterRequest) error {
		return bizerrors.ErrVerifyCodeInvalid
	}

	body := JSONBody(request.RegisterRequest{
		Email:           "new@example.com",
		Password:        "Test123456",
		ConfirmPassword: "Test123456",
		Code:            "000000",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/register", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeVerifyCodeInvalid)
}

// ======================== TC-004: 用户登录 ========================

func TestLogin_Success(t *testing.T) {
	env := NewTestEnv()
	env.Auth.LoginFunc = func(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error) {
		return &dto.LoginResponse{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			User: dto.UserResponse{
				ID:       1,
				Username: "testuser",
				Email:    req.Email,
				Status:   1,
			},
		}, nil
	}

	body := JSONBody(request.LoginRequest{
		Email:     "test@example.com",
		Password:  "Test123456",
		CaptchaID: "captcha-id",
		CaptchaX:  150,
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/login", body, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["access_token"] == nil {
		t.Error("access_token 不能为空")
	}
	if data["refresh_token"] == nil {
		t.Error("refresh_token 不能为空")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.LoginRequest{
		Email:     "test@example.com",
		Password:  "wrong-password",
		CaptchaID: "captcha-id",
		CaptchaX:  150,
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/login", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInvalidCredentials)
}

func TestLogin_UserDisabled(t *testing.T) {
	env := NewTestEnv()
	env.Auth.LoginFunc = func(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error) {
		return nil, bizerrors.ErrUserDisabled
	}

	body := JSONBody(request.LoginRequest{
		Email:     "disabled@example.com",
		Password:  "Test123456",
		CaptchaID: "captcha-id",
		CaptchaX:  150,
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/login", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUserDisabled)
}

func TestLogin_MissingFields(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"email": "test@example.com"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/login", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-005: 刷新 Token ========================

func TestRefreshToken_Success(t *testing.T) {
	env := NewTestEnv()
	env.Auth.RefreshTokenFunc = func(ctx context.Context, refreshToken string) (*dto.LoginResponse, error) {
		return &dto.LoginResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			User: dto.UserResponse{
				ID:       1,
				Username: "testuser",
				Email:    "test@example.com",
			},
		}, nil
	}

	body := JSONBody(gin.H{"refresh_token": "valid-refresh-token"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/refresh", body, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["access_token"] == nil {
		t.Error("access_token 不能为空")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"refresh_token": "invalid-token"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/refresh", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInvalidToken)
}

func TestRefreshToken_ExpiredToken(t *testing.T) {
	env := NewTestEnv()
	env.Auth.RefreshTokenFunc = func(ctx context.Context, refreshToken string) (*dto.LoginResponse, error) {
		return nil, bizerrors.ErrTokenExpired
	}

	body := JSONBody(gin.H{"refresh_token": "expired-token"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/refresh", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeTokenExpired)
}

func TestRefreshToken_MissingField(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/refresh", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-006: 用户登出 ========================

func TestLogout_Success(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{
		"access_token":  "some-access-token",
		"refresh_token": "some-refresh-token",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/logout", body, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestLogout_Idempotent(t *testing.T) {
	env := NewTestEnv()
	env.Auth.LogoutFunc = func(ctx context.Context, accessToken string, refreshToken string) error {
		// 幂等：已失效的 token 也返回成功
		return nil
	}

	body := JSONBody(gin.H{
		"access_token":  "already-invalid-token",
		"refresh_token": "already-invalid-token",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/logout", body, "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestLogout_InvalidToken(t *testing.T) {
	env := NewTestEnv()
	env.Auth.LogoutFunc = func(ctx context.Context, accessToken string, refreshToken string) error {
		return bizerrors.ErrInvalidToken
	}

	body := JSONBody(gin.H{
		"access_token":  "invalid",
		"refresh_token": "invalid",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/logout", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInvalidToken)
}

// ======================== 辅助：无 Body 的 POST 请求 ========================

func TestLogout_EmptyBody(t *testing.T) {
	env := NewTestEnv()

	// 空 body 也能成功（logout 是幂等的）
	w := MakeRequest(env.Engine, "POST", "/api/v1/auth/logout", []byte(`{}`), "")
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

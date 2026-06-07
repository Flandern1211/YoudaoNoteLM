package integration

import (
	"mime/multipart"
	"testing"
	"time"

	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// ======================== TC-007: 获取用户信息 ========================

func TestGetProfile_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	env.User.GetUserByIDFunc = func(id uint) (*entity.User, error) {
		return &entity.User{
			BaseEntity: entity.BaseEntity{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Username:   "testuser",
			Email:      "test@example.com",
			Status:     1,
		}, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/profile", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["username"] != "testuser" {
		t.Errorf("期望 username=testuser, 实际=%v", data["username"])
	}
	if data["email"] != "test@example.com" {
		t.Errorf("期望 email=test@example.com, 实际=%v", data["email"])
	}
}

func TestGetProfile_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/profile", nil, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

func TestGetProfile_InvalidToken(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/profile", nil, "invalid-token")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInvalidToken)
}

func TestGetProfile_UserNotFound(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 999, "ghost")

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/profile", nil, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUserNotFound)
}

// ======================== TC-008: 更新用户信息 ========================

func TestUpdateProfile_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.UpdateUserRequest{
		Nickname: "新昵称",
	})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/profile", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestUpdateProfile_NicknameTooLong(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	// nickname max=50
	longNickname := ""
	for i := 0; i < 51; i++ {
		longNickname += "a"
	}
	body := JSONBody(gin.H{"nickname": longNickname})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/profile", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestUpdateProfile_InvalidAvatarURL(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"avatar": "not-a-url"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/profile", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"nickname": "只改昵称"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/profile", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestUpdateProfile_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"nickname": "test"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/profile", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-009: 修改用户名 ========================

func TestUpdateUsername_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.UpdateUsernameRequest{Username: "newusername"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/username", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestUpdateUsername_AlreadyExists(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.User.UpdateUsernameFunc = func(id uint, req *request.UpdateUsernameRequest) error {
		return bizerrors.ErrUserAlreadyExists
	}

	body := JSONBody(request.UpdateUsernameRequest{Username: "existinguser"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/username", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUserAlreadyExists)
}

func TestUpdateUsername_TooShort(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"username": "ab"})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/username", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestUpdateUsername_TooLong(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	longName := ""
	for i := 0; i < 51; i++ {
		longName += "a"
	}
	body := JSONBody(gin.H{"username": longName})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/username", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestUpdateUsername_Empty(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"username": ""})
	w := MakeRequest(env.Engine, "PUT", "/api/v1/user/username", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-010: 上传头像 ========================

func TestUploadAvatar_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.User.UploadAvatarFunc = func(id uint, file *multipart.FileHeader) (string, error) {
		return "/uploads/avatars/1_avatar.jpg", nil
	}

	w := MakeFormRequest(env.Engine, "POST", "/api/v1/user/avatar", "avatar", "avatar.jpg", []byte("fake-image-data"), token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestUploadAvatar_NoFile(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	req := newRequest("POST", "/api/v1/user/avatar", nil, token)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := doRequest(env.Engine, req)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

// ======================== TC-011: 修改密码 ========================

func TestChangePassword_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.ChangePasswordRequest{
		OldPassword: "Test123456",
		NewPassword: "NewTest123456",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/user/password", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.User.ChangePasswordFunc = func(id uint, req *request.ChangePasswordRequest) error {
		return bizerrors.ErrInvalidCredentials
	}

	body := JSONBody(request.ChangePasswordRequest{
		OldPassword: "wrong",
		NewPassword: "NewTest123456",
	})
	w := MakeRequest(env.Engine, "POST", "/api/v1/user/password", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInvalidCredentials)
}

func TestChangePassword_NewPasswordTooShort(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(gin.H{"old_password": "Test123456", "new_password": "123"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/user/password", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeBadRequest)
}

func TestChangePassword_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(gin.H{"old_password": "Test123456", "new_password": "NewTest123456"})
	w := MakeRequest(env.Engine, "POST", "/api/v1/user/password", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-012: 注销账号 ========================

func TestDeleteAccount_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")

	body := JSONBody(request.DeleteAccountRequest{
		Password: "Test123456",
		Code:     "123456",
	})
	w := MakeRequest(env.Engine, "DELETE", "/api/v1/user/account", body, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
}

func TestDeleteAccount_WrongPassword(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.User.DeleteAccountFunc = func(id uint, req *request.DeleteAccountRequest) error {
		return bizerrors.ErrInvalidCredentials
	}

	body := JSONBody(request.DeleteAccountRequest{
		Password: "wrong",
		Code:     "123456",
	})
	w := MakeRequest(env.Engine, "DELETE", "/api/v1/user/account", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeInvalidCredentials)
}

func TestDeleteAccount_InvalidCode(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "testuser")
	env.User.DeleteAccountFunc = func(id uint, req *request.DeleteAccountRequest) error {
		return bizerrors.ErrVerifyCodeInvalid
	}

	body := JSONBody(request.DeleteAccountRequest{
		Password: "Test123456",
		Code:     "000000",
	})
	w := MakeRequest(env.Engine, "DELETE", "/api/v1/user/account", body, token)
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeVerifyCodeInvalid)
}

func TestDeleteAccount_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	body := JSONBody(request.DeleteAccountRequest{
		Password: "Test123456",
		Code:     "123456",
	})
	w := MakeRequest(env.Engine, "DELETE", "/api/v1/user/account", body, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

// ======================== TC-013: 用户列表 ========================

func TestListUsers_Success(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "admin")
	env.User.ListUsersFunc = func(req *request.UserListRequest) (*response.PageResponse, error) {
		return &response.PageResponse{
			List: []dto.UserResponse{
				{ID: 1, Username: "user1", Email: "u1@example.com", Status: 1},
				{ID: 2, Username: "user2", Email: "u2@example.com", Status: 1},
			},
			Total:     2,
			Page:      1,
			Size:      20,
			TotalPage: 1,
		}, nil
	}

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/list?page=1&size=20", nil, token)
	result := ParseResponse(t, w)

	AssertSuccess(t, result)
	data := GetDataMap(t, result)
	if data["total"].(float64) != 2 {
		t.Errorf("期望 total=2, 实际=%v", data["total"])
	}
}

func TestListUsers_DefaultPagination(t *testing.T) {
	env := NewTestEnv()
	token := GenerateTestToken(t, 1, "admin")

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/list", nil, token)
	// 无分页参数可能返回 400（取决于 binding tag）
	result := ParseResponse(t, w)
	code := result["code"].(float64)
	if code != 0 && code != 400 {
		t.Errorf("期望 code=0 或 400, 实际=%.0f", code)
	}
}

func TestListUsers_Unauthorized(t *testing.T) {
	env := NewTestEnv()

	w := MakeRequest(env.Engine, "GET", "/api/v1/user/list?page=1&size=20", nil, "")
	result := ParseResponse(t, w)

	AssertCode(t, result, bizerrors.CodeUnauthorized)
}

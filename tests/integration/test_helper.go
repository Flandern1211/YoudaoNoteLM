package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"YoudaoNoteLm/internal/api"
	"YoudaoNoteLm/internal/model/dto/request"
	dto "YoudaoNoteLm/internal/model/dto/response"
	"YoudaoNoteLm/internal/model/entity"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/config"
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/jwt"
	"YoudaoNoteLm/pkg/logger"
	pkgresponse "YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
	config.SetForTest(&config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-secret-key-for-unit-tests",
			AccessTokenExp: "15m",
			RefreshTokenExp: "7d",
			Issuer:         "youdaonotelm-test",
		},
	})
	// 初始化 logger（middleware 需要）
	_ = logger.Init(&config.LogConfig{
		Level:    "error",
		Filename: "test.log",
	})
}

// ======================== 测试环境 ========================

// TestEnv 测试环境，持有所有 mock service
type TestEnv struct {
	Engine    *gin.Engine
	Auth      *MockAuthService
	User      *MockUserService
	Notebook  *MockNotebookService
	Source    *MockSourceService
	Importer  *MockImporterService
	Search    *MockSearchAgentService
	Captcha   *MockCaptchaService
	Blacklist *MockTokenBlacklistService
}

// NewTestEnv 创建测试环境，注册所有路由
func NewTestEnv() *TestEnv {
	env := &TestEnv{
		Auth:      &MockAuthService{},
		User:      &MockUserService{},
		Notebook:  &MockNotebookService{},
		Source:    &MockSourceService{},
		Importer:  &MockImporterService{},
		Search:    &MockSearchAgentService{},
		Captcha:   &MockCaptchaService{},
		Blacklist: &MockTokenBlacklistService{},
	}

	router := api.NewRouter(
		env.User,
		env.Auth,
		env.Notebook,
		env.Source,
		env.Importer,
		env.Search,
		env.Captcha,
		env.Blacklist,
	)

	engine := gin.New()
	router.Setup(engine)
	env.Engine = engine

	return env
}

// ======================== 辅助函数 ========================

// GenerateTestToken 生成测试用 JWT access token
func GenerateTestToken(t *testing.T, userID uint, username string) string {
	t.Helper()
	token, err := jwt.GenerateAccessToken(userID, username)
	if err != nil {
		t.Fatalf("生成测试 token 失败: %v", err)
	}
	return token
}

// JSONBody 将对象序列化为 JSON bytes
func JSONBody(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// MakeRequest 发送 JSON 请求
func MakeRequest(engine *gin.Engine, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader([]byte{})
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// newRequest 创建带 token 的 HTTP 请求
func newRequest(method, path string, body []byte, token string) *http.Request {
	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader([]byte{})
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// doRequest 执行请求并返回响应
func doRequest(engine *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// MakeFormRequest 发送 multipart/form-data 文件上传请求
func MakeFormRequest(engine *gin.Engine, method, path, fieldName, fileName string, fileContent []byte, token string) *httptest.ResponseRecorder {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, fileName)
	_, _ = part.Write(fileContent)
	writer.Close()

	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// ParseResponse 解析 JSON 响应为 map
func ParseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v, body: %s", err, w.Body.String())
	}
	return result
}

// AssertCode 断言业务错误码
func AssertCode(t *testing.T, result map[string]interface{}, expected int) {
	t.Helper()
	code, ok := result["code"].(float64)
	if !ok {
		t.Fatalf("响应中没有 code 字段: %v", result)
	}
	if int(code) != expected {
		t.Errorf("期望 code=%d, 实际 code=%.0f, message=%v", expected, code, result["message"])
	}
}

// AssertSuccess 断言成功响应 (code=0)
func AssertSuccess(t *testing.T, result map[string]interface{}) {
	t.Helper()
	AssertCode(t, result, bizerrors.CodeSuccess)
}

// AssertMessageContains 断言 message 包含子串
func AssertMessageContains(t *testing.T, result map[string]interface{}, substr string) {
	t.Helper()
	msg, _ := result["message"].(string)
	if !strings.Contains(msg, substr) {
		t.Errorf("期望 message 包含 %q, 实际=%q", substr, msg)
	}
}

// GetDataMap 从响应中提取 data 为 map
func GetDataMap(t *testing.T, result map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是 map 类型: %T, value=%v", result["data"], result["data"])
	}
	return data
}

// GetDataArray 从响应中提取 data 为数组
func GetDataArray(t *testing.T, result map[string]interface{}) []interface{} {
	t.Helper()
	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatalf("data 不是数组类型: %T, value=%v", result["data"], result["data"])
	}
	return data
}

// ======================== Mock TokenBlacklistService ========================

type MockTokenBlacklistService struct {
	RevokeTokenFunc func(ctx context.Context, tokenString string) error
	IsRevokedFunc   func(ctx context.Context, jti string) (bool, error)
}

var _ service.TokenBlacklistService = (*MockTokenBlacklistService)(nil)

func (m *MockTokenBlacklistService) RevokeToken(ctx context.Context, tokenString string) error {
	if m.RevokeTokenFunc != nil {
		return m.RevokeTokenFunc(ctx, tokenString)
	}
	return nil
}

func (m *MockTokenBlacklistService) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if m.IsRevokedFunc != nil {
		return m.IsRevokedFunc(ctx, jti)
	}
	return false, nil
}

// ======================== Mock CaptchaService ========================

type MockCaptchaService struct {
	GenerateFunc func(ctx context.Context) (*dto.CaptchaData, error)
	VerifyFunc   func(ctx context.Context, captchaID string, userX int) error
}

var _ service.CaptchaService = (*MockCaptchaService)(nil)

func (m *MockCaptchaService) Generate(ctx context.Context) (*dto.CaptchaData, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx)
	}
	return &dto.CaptchaData{
		CaptchaID:    "test-captcha-id",
		Background:   "base64-bg",
		Slider:       "base64-slider",
		SliderSize:   50,
		BgWidth:      300,
		SliderStartX: 10,
	}, nil
}

func (m *MockCaptchaService) Verify(ctx context.Context, captchaID string, userX int) error {
	if m.VerifyFunc != nil {
		return m.VerifyFunc(ctx, captchaID, userX)
	}
	return nil
}

// ======================== Mock AuthService ========================

type MockAuthService struct {
	LoginFunc        func(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error)
	RefreshTokenFunc func(ctx context.Context, refreshToken string) (*dto.LoginResponse, error)
	LogoutFunc       func(ctx context.Context, accessToken string, refreshToken string) error
	SendCodeFunc     func(ctx context.Context, req *request.SendCodeRequest) (*dto.SendCodeResponse, error)
	ResetPasswordFunc func(req *request.ResetPasswordRequest) error
}

var _ service.AuthService = (*MockAuthService)(nil)

func (m *MockAuthService) Login(ctx context.Context, req *request.LoginRequest) (*dto.LoginResponse, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, req)
	}
	return nil, bizerrors.ErrInvalidCredentials
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*dto.LoginResponse, error) {
	if m.RefreshTokenFunc != nil {
		return m.RefreshTokenFunc(ctx, refreshToken)
	}
	return nil, bizerrors.ErrInvalidToken
}

func (m *MockAuthService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	if m.LogoutFunc != nil {
		return m.LogoutFunc(ctx, accessToken, refreshToken)
	}
	return nil
}

func (m *MockAuthService) SendCode(ctx context.Context, req *request.SendCodeRequest) (*dto.SendCodeResponse, error) {
	if m.SendCodeFunc != nil {
		return m.SendCodeFunc(ctx, req)
	}
	return &dto.SendCodeResponse{RetryAfter: 60}, nil
}

func (m *MockAuthService) ResetPassword(req *request.ResetPasswordRequest) error {
	if m.ResetPasswordFunc != nil {
		return m.ResetPasswordFunc(req)
	}
	return nil
}

// ======================== Mock UserService ========================

type MockUserService struct {
	RegisterFunc       func(ctx context.Context, req *request.RegisterRequest) error
	GetUserByIDFunc    func(id uint) (*entity.User, error)
	UpdateUserFunc     func(id uint, req *request.UpdateUserRequest) error
	UpdateUsernameFunc func(id uint, req *request.UpdateUsernameRequest) error
	UploadAvatarFunc   func(id uint, file *multipart.FileHeader) (string, error)
	ChangePasswordFunc func(id uint, req *request.ChangePasswordRequest) error
	DeleteAccountFunc  func(id uint, req *request.DeleteAccountRequest) error
	GetUserResponseFunc func(user *entity.User) *dto.UserResponse
	ListUsersFunc      func(req *request.UserListRequest) (*pkgresponse.PageResponse, error)
}

var _ service.UserService = (*MockUserService)(nil)

func (m *MockUserService) Register(ctx context.Context, req *request.RegisterRequest) error {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, req)
	}
	return nil
}

func (m *MockUserService) GetUserByID(id uint) (*entity.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(id)
	}
	return nil, bizerrors.ErrUserNotFound
}

func (m *MockUserService) UpdateUser(id uint, req *request.UpdateUserRequest) error {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(id, req)
	}
	return nil
}

func (m *MockUserService) UpdateUsername(id uint, req *request.UpdateUsernameRequest) error {
	if m.UpdateUsernameFunc != nil {
		return m.UpdateUsernameFunc(id, req)
	}
	return nil
}

func (m *MockUserService) UploadAvatar(id uint, file *multipart.FileHeader) (string, error) {
	if m.UploadAvatarFunc != nil {
		return m.UploadAvatarFunc(id, file)
	}
	return "", nil
}

func (m *MockUserService) ChangePassword(id uint, req *request.ChangePasswordRequest) error {
	if m.ChangePasswordFunc != nil {
		return m.ChangePasswordFunc(id, req)
	}
	return nil
}

func (m *MockUserService) DeleteAccount(id uint, req *request.DeleteAccountRequest) error {
	if m.DeleteAccountFunc != nil {
		return m.DeleteAccountFunc(id, req)
	}
	return nil
}

func (m *MockUserService) GetUserResponse(user *entity.User) *dto.UserResponse {
	if m.GetUserResponseFunc != nil {
		return m.GetUserResponseFunc(user)
	}
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

func (m *MockUserService) ListUsers(req *request.UserListRequest) (*pkgresponse.PageResponse, error) {
	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(req)
	}
	return &pkgresponse.PageResponse{
		List:      []dto.UserResponse{},
		Total:     0,
		Page:      req.Page,
		Size:      req.Size,
		TotalPage: 0,
	}, nil
}

// ======================== Mock NotebookService ========================

type MockNotebookService struct {
	CreateFunc func(userID uint, req *request.CreateNotebookRequest) (*dto.NotebookResponse, error)
	ListFunc   func(userID uint) ([]*dto.NotebookResponse, error)
	RenameFunc func(userID, notebookID uint, req *request.RenameNotebookRequest) error
	DeleteFunc func(userID, notebookID uint) error
}

var _ service.NotebookService = (*MockNotebookService)(nil)

func (m *MockNotebookService) Create(userID uint, req *request.CreateNotebookRequest) (*dto.NotebookResponse, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(userID, req)
	}
	return &dto.NotebookResponse{
		ID:   1,
		Name: req.Name,
	}, nil
}

func (m *MockNotebookService) List(userID uint) ([]*dto.NotebookResponse, error) {
	if m.ListFunc != nil {
		return m.ListFunc(userID)
	}
	return []*dto.NotebookResponse{}, nil
}

func (m *MockNotebookService) Rename(userID, notebookID uint, req *request.RenameNotebookRequest) error {
	if m.RenameFunc != nil {
		return m.RenameFunc(userID, notebookID, req)
	}
	return nil
}

func (m *MockNotebookService) Delete(userID, notebookID uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(userID, notebookID)
	}
	return nil
}

// ======================== Mock SourceService ========================

type MockSourceService struct {
	ListFunc              func(userID, notebookID uint, keyword string, page, size int) ([]*dto.SourceResponse, int64, error)
	GetByIDFunc           func(id uint) (*entity.Source, error)
	RenameFunc            func(id uint, name string) error
	DeleteFunc            func(id uint) error
	BatchDeleteFunc       func(ids []uint) error
	GetContentFunc        func(id uint) (string, error)
	GetOriginalContentFunc func(id uint) (string, string, error)
	GetDownloadURLFunc    func(id uint) (string, error)
}

var _ service.SourceService = (*MockSourceService)(nil)

func (m *MockSourceService) List(userID, notebookID uint, keyword string, page, size int) ([]*dto.SourceResponse, int64, error) {
	if m.ListFunc != nil {
		return m.ListFunc(userID, notebookID, keyword, page, size)
	}
	return []*dto.SourceResponse{}, 0, nil
}

func (m *MockSourceService) GetByID(id uint) (*entity.Source, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, bizerrors.ErrNotFound
}

func (m *MockSourceService) Rename(id uint, name string) error {
	if m.RenameFunc != nil {
		return m.RenameFunc(id, name)
	}
	return nil
}

func (m *MockSourceService) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *MockSourceService) BatchDelete(ids []uint) error {
	if m.BatchDeleteFunc != nil {
		return m.BatchDeleteFunc(ids)
	}
	return nil
}

func (m *MockSourceService) GetContent(id uint) (string, error) {
	if m.GetContentFunc != nil {
		return m.GetContentFunc(id)
	}
	return "", nil
}

func (m *MockSourceService) GetOriginalContent(id uint) (string, string, error) {
	if m.GetOriginalContentFunc != nil {
		return m.GetOriginalContentFunc(id)
	}
	return "", "", nil
}

func (m *MockSourceService) GetDownloadURL(id uint) (string, error) {
	if m.GetDownloadURLFunc != nil {
		return m.GetDownloadURLFunc(id)
	}
	return "", nil
}

// ======================== Mock ImporterService ========================

type MockImporterService struct {
	ImportFileFunc          func(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error)
	PreviewAudioFunc        func(userID, notebookID uint, file *multipart.FileHeader) (string, string, string, error)
	ConfirmAudioFunc        func(userID uint, previewID string, editedContent *string) (*entity.Source, error)
	ImportSearchResultsFunc func(userID, notebookID uint, urls []string) (string, error)
	GetImportTaskFunc       func(taskID string) (interface{}, error)
}

var _ service.ImporterService = (*MockImporterService)(nil)

func (m *MockImporterService) ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error) {
	if m.ImportFileFunc != nil {
		return m.ImportFileFunc(userID, notebookID, file)
	}
	return nil, nil
}

func (m *MockImporterService) PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (string, string, string, error) {
	if m.PreviewAudioFunc != nil {
		return m.PreviewAudioFunc(userID, notebookID, file)
	}
	return "", "", "", nil
}

func (m *MockImporterService) ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error) {
	if m.ConfirmAudioFunc != nil {
		return m.ConfirmAudioFunc(userID, previewID, editedContent)
	}
	return nil, nil
}

func (m *MockImporterService) ImportSearchResults(userID, notebookID uint, urls []string) (string, error) {
	if m.ImportSearchResultsFunc != nil {
		return m.ImportSearchResultsFunc(userID, notebookID, urls)
	}
	return "", nil
}

func (m *MockImporterService) GetImportTask(taskID string) (interface{}, error) {
	if m.GetImportTaskFunc != nil {
		return m.GetImportTaskFunc(taskID)
	}
	return nil, nil
}

// ======================== Mock SearchAgentService ========================

type MockSearchAgentService struct {
	SearchFunc              func(userID, notebookID uint, query string) (*dto.SearchResponse, error)
	ImportFromURLFunc       func(userID, notebookID uint, url string) (string, error)
	ImportSearchResultsFunc func(userID, notebookID uint, urls []string) (string, error)
}

var _ service.SearchAgentService = (*MockSearchAgentService)(nil)

func (m *MockSearchAgentService) Search(userID, notebookID uint, query string) (*dto.SearchResponse, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(userID, notebookID, query)
	}
	return nil, nil
}

func (m *MockSearchAgentService) ImportFromURL(userID, notebookID uint, url string) (string, error) {
	if m.ImportFromURLFunc != nil {
		return m.ImportFromURLFunc(userID, notebookID, url)
	}
	return "", nil
}

func (m *MockSearchAgentService) ImportSearchResults(userID, notebookID uint, urls []string) (string, error) {
	if m.ImportSearchResultsFunc != nil {
		return m.ImportSearchResultsFunc(userID, notebookID, urls)
	}
	return "", nil
}

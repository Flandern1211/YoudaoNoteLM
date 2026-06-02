package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`    // 业务状态码
	Message string      `json:"message"` // 响应消息
	Data    interface{} `json:"data"`    // 响应数据
}

// PageData 分页数据结构
type PageData struct {
	List     interface{} `json:"list"`      // 数据列表
	Total    int64       `json:"total"`     // 总数
	Page     int         `json:"page"`      // 当前页码
	PageSize int         `json:"page_size"` // 每页数量
}

// 业务状态码定义
const (
	CodeSuccess       = 0   // 成功
	CodeBadRequest    = 400 // 请求参数错误
	CodeUnauthorized  = 401 // 未授权
	CodeForbidden     = 403 // 禁止访问
	CodeNotFound      = 404 // 资源不存在
	CodeConflict      = 409 // 资源冲突
	CodeInternalError = 500 // 内部服务器错误

	// 认证相关 1xxx
	CodeInvalidCredentials = 1001 // 用户名或密码错误
	CodeAccountLocked      = 1002 // 账户已锁定
	CodeTokenExpired       = 1003 // Token 已过期
	CodeTokenInvalid       = 1004 // Token 无效
	CodeVerifyCodeError    = 1005 // 验证码错误
	CodeEmailAlreadyExists = 1006 // 邮箱已注册

	// 笔记本相关 2xxx
	CodeNotebookNotFound = 2001 // 笔记本不存在

	// 资料来源相关 3xxx
	CodeSourceNotFound    = 3001 // 资料来源不存在
	CodeSourceImportFail  = 3002 // 资料导入失败
	CodeFileTooLarge      = 3003 // 文件过大
	CodeUnsupportedFormat = 3004 // 不支持的格式

	// 对话相关 4xxx
	CodeConversationNotFound = 4001 // 对话不存在

	// 生成相关 5xxx
	CodeGenerationFail   = 5001 // 生成失败
	CodeNoSourceSelected = 5002 // 未选择资料来源
	CodeInvalidGenType   = 5003 // 无效的生成类型

	// AI 配置相关 6xxx
	CodeAIConfigNotFound = 6001 // AI 配置不存在
	CodeAIConfigInvalid  = 6002 // AI 配置无效
	CodeAIConfigTestFail = 6003 // AI 配置测试失败
)

// 状态码对应的消息映射
var codeMessages = map[int]string{
	CodeSuccess:       "success",
	CodeBadRequest:    "请求参数错误",
	CodeUnauthorized:  "未授权",
	CodeForbidden:     "禁止访问",
	CodeNotFound:      "资源不存在",
	CodeConflict:      "资源冲突",
	CodeInternalError: "内部服务器错误",

	CodeInvalidCredentials: "用户名或密码错误",
	CodeAccountLocked:      "账户已锁定，请稍后再试",
	CodeTokenExpired:       "Token 已过期",
	CodeTokenInvalid:       "Token 无效",
	CodeVerifyCodeError:    "验证码错误",
	CodeEmailAlreadyExists: "邮箱已注册",

	CodeNotebookNotFound: "笔记本不存在",

	CodeSourceNotFound:    "资料来源不存在",
	CodeSourceImportFail:  "资料导入失败",
	CodeFileTooLarge:      "文件过大",
	CodeUnsupportedFormat: "不支持的格式",

	CodeConversationNotFound: "对话不存在",

	CodeGenerationFail:   "生成失败",
	CodeNoSourceSelected: "请先选择资料来源",
	CodeInvalidGenType:   "无效的生成类型",

	CodeAIConfigNotFound: "AI 配置不存在",
	CodeAIConfigInvalid:  "AI 配置无效",
	CodeAIConfigTestFail: "AI 配置测试失败",
}

// GetMessage 根据状态码获取消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// JSON 返回 JSON 响应
func JSON(c *gin.Context, httpStatus int, code int, data interface{}) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: GetMessage(code),
		Data:    data,
	})
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, CodeSuccess, data)
}

// SuccessWithMessage 返回成功响应（自定义消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// SuccessPage 返回分页成功响应
func SuccessPage(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	Success(c, PageData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// Fail 返回失败响应
func Fail(c *gin.Context, code int) {
	JSON(c, http.StatusOK, code, nil)
}

// FailWithMessage 返回失败响应（自定义消息）
func FailWithMessage(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// BadRequest 返回 400 响应
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    CodeBadRequest,
		Message: message,
		Data:    nil,
	})
}

// Unauthorized 返回 401 响应
func Unauthorized(c *gin.Context) {
	JSON(c, http.StatusUnauthorized, CodeUnauthorized, nil)
}

// Forbidden 返回 403 响应
func Forbidden(c *gin.Context) {
	JSON(c, http.StatusForbidden, CodeForbidden, nil)
}

// NotFound 返回 404 响应
func NotFound(c *gin.Context) {
	JSON(c, http.StatusNotFound, CodeNotFound, nil)
}

// InternalError 返回 500 响应
func InternalError(c *gin.Context) {
	JSON(c, http.StatusInternalServerError, CodeInternalError, nil)
}

// InternalErrorWithMessage 返回 500 响应（自定义消息）
func InternalErrorWithMessage(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    CodeInternalError,
		Message: message,
		Data:    nil,
	})
}

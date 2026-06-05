// internal/api/v1/search/controller.go
package search

import (
	"strconv"

	"YoudaoNoteLm/internal/middleware"
	"YoudaoNoteLm/internal/model/dto/request"
	"YoudaoNoteLm/internal/service"
	"YoudaoNoteLm/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 搜索控制器
type Controller struct {
	searchService    service.SearchAgentService
	tokenBlacklist   service.TokenBlacklistService
}

// NewController 创建搜索控制器
func NewController(searchService service.SearchAgentService, tokenBlacklist service.TokenBlacklistService) *Controller {
	return &Controller{searchService: searchService, tokenBlacklist: tokenBlacklist}
}

// Search 智能搜索
func (ctrl *Controller) Search(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := ctrl.searchService.Search(userID, uint(nbID), req.Query)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, result)
}

// ImportFromURL URL 直接导入
func (ctrl *Controller) ImportFromURL(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.URLImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	source, err := ctrl.searchService.ImportFromURL(userID, uint(nbID), req.URL)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"source": source})
}

// ImportSearchResults 批量导入搜索结果
func (ctrl *Controller) ImportSearchResults(c *gin.Context) {
	userID := middleware.GetUserID(c)
	nbID, err := strconv.ParseUint(c.Param("nbId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的笔记本ID")
		return
	}

	var req request.SearchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	taskID, err := ctrl.searchService.ImportSearchResults(userID, uint(nbID), req.URLs)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"task_id": taskID})
}

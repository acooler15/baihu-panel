package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type TagController struct {
	tagService *services.TagService
}

func NewTagController(tagService *services.TagService) *TagController {
	return &TagController{
		tagService: tagService,
	}
}

// GetTags 获取标签列表 (带关联统计)
// 已迁移至 Huma：TAGetTags /api/v1/tags
func (tc *TagController) GetTags(c *gin.Context) {
	p := utils.ParsePagination(c)
	name := c.Query("name")
	relType := c.Query("type")

	tags, total, err := tc.tagService.GetTagsWithPagination(p.Page, p.PageSize, name, relType)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.PaginatedResponse(c, tags, total, p)
}

// CreateTag 手动新建标签
// 已迁移至 Huma：TACreateTag /api/v1/tags
func (tc *TagController) CreateTag(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	tag, err := tc.tagService.CreateTag(req.Name, req.Type)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, tag)
}

// UpdateTag 重命名标签
// 已迁移至 Huma：TAUpdateTag /api/v1/tags/{id}
func (tc *TagController) UpdateTag(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if err := tc.tagService.RenameTag(id, req.Name); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMsg(c, "更新成功")
}

// DeleteTag 删除标签
// 已迁移至 Huma：TADeleteTag /api/v1/tags/{id}
func (tc *TagController) DeleteTag(c *gin.Context) {
	id := c.Param("id")
	if err := tc.tagService.DeleteTag(id); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMsg(c, "删除成功")
}

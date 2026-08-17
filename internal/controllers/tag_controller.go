package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type TagController struct {
	tagService *services.TagService
}

func NewTagController(tagService *services.TagService) *TagController {
	return &TagController{
		tagService: tagService,
	}
}

// ===========================================================================
// 标签管理业务方法
// ===========================================================================

// GetTagsInput 获取标签列表
type GetTagsInput struct {
	Name     string `query:"name" description:"标签名模糊搜索"`
	Type     string `query:"type" description:"标签类型 (task_tag, env_tag)"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// GetTagsOutput 获取标签列表
type GetTagsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]services.TagWithCount]]
}

// GetTags 获取标签列表（带关联统计）
func (tc *TagController) GetTags(ctx context.Context, input *GetTagsInput) (*GetTagsOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	tags, total, err := tc.tagService.GetTagsWithPagination(page, pageSize, input.Name, input.Type)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &GetTagsOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]services.TagWithCount]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]services.TagWithCount]{
				Data:     tags,
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// CreateTagInput 新建标签
type CreateTagInput struct {
	Body struct {
		Name string `json:"name" description:"标签名称"`
		Type string `json:"type" description:"标签类型"`
	}
}

// CreateTagOutput 新建标签
type CreateTagOutput struct {
	Body utils.HumaResponse[*models.DataStorage]
}

// CreateTag 新建标签
func (tc *TagController) CreateTag(ctx context.Context, input *CreateTagInput) (*CreateTagOutput, error) {
	req := input.Body
	if req.Name == "" {
		return nil, utils.HumaBadRequest("标签名称不能为空")
	}
	if req.Type == "" {
		return nil, utils.HumaBadRequest("标签类型不能为空")
	}

	tag, err := tc.tagService.CreateTag(req.Name, req.Type)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &CreateTagOutput{
		Body: utils.HumaResponse[*models.DataStorage]{
			Code: 200,
			Msg:  "success",
			Data: tag,
		},
	}, nil
}

// UpdateTagInput 修改标签
type UpdateTagInput struct {
	ID   string `path:"id" description:"标签ID"`
	Body struct {
		Name string `json:"name" description:"新标签名称"`
	}
}

// UpdateTagOutput 修改标签
type UpdateTagOutput struct {
	Body utils.HumaResponse[any]
}

// UpdateTag 重命名标签
func (tc *TagController) UpdateTag(ctx context.Context, input *UpdateTagInput) (*UpdateTagOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的标签ID")
	}
	if input.Body.Name == "" {
		return nil, utils.HumaBadRequest("标签名称不能为空")
	}

	if err := tc.tagService.RenameTag(input.ID, input.Body.Name); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &UpdateTagOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "更新成功",
		},
	}, nil
}

// DeleteTagInput 删除标签
type DeleteTagInput struct {
	ID string `path:"id" description:"标签ID"`
}

// DeleteTagOutput 删除标签
type DeleteTagOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteTag 删除标签
func (tc *TagController) DeleteTag(ctx context.Context, input *DeleteTagInput) (*DeleteTagOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的标签ID")
	}

	if err := tc.tagService.DeleteTag(input.ID); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &DeleteTagOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPITagRoutes 注册 /api/v1 标签管理 Huma 路由
func (tc *TagController) RegisterAPITagRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"标签管理"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/tags", OperationID: "GetTags", Summary: "获取标签列表", Description: "获取标签列表，支持分页、模糊搜索、类型过滤以及统计关联的实体数", Tags: tag, Security: security}, tc.GetTags)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tags", OperationID: "CreateTag", Summary: "新建标签", Description: "手动新建标签", Tags: tag, Security: security}, tc.CreateTag)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/tags/{id}", OperationID: "UpdateTag", Summary: "修改标签", Description: "重命名标签名称", Tags: tag, Security: security}, tc.UpdateTag)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/tags/{id}", OperationID: "DeleteTag", Summary: "删除标签", Description: "删除标签，并清理其所有对应绑定关系", Tags: tag, Security: security}, tc.DeleteTag)
}

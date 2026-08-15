package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 标签管理
// ===========================================================================

// TAGetTagsInput 获取标签列表
type TAGetTagsInput struct {
	Name     string `query:"name" description:"标签名模糊搜索"`
	Type     string `query:"type" description:"标签类型 (task_tag, env_tag)"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// TAGetTagsOutput 获取标签列表
type TAGetTagsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]services.TagWithCount]]
}

// TAGetTags 获取标签列表（带关联统计）
func (tc *TagController) TAGetTags(ctx context.Context, input *TAGetTagsInput) (*TAGetTagsOutput, error) {
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

	return &TAGetTagsOutput{
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

// TACreateTagInput 新建标签
type TACreateTagInput struct {
	Body struct {
		Name string `json:"name" description:"标签名称"`
		Type string `json:"type" description:"标签类型"`
	}
}

// TACreateTagOutput 新建标签
type TACreateTagOutput struct {
	Body utils.HumaResponse[*models.DataStorage]
}

// TACreateTag 新建标签
func (tc *TagController) TACreateTag(ctx context.Context, input *TACreateTagInput) (*TACreateTagOutput, error) {
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

	return &TACreateTagOutput{
		Body: utils.HumaResponse[*models.DataStorage]{
			Code: 200,
			Msg:  "success",
			Data: tag,
		},
	}, nil
}

// TAUpdateTagInput 修改标签
type TAUpdateTagInput struct {
	ID   string `path:"id" description:"标签ID"`
	Body struct {
		Name string `json:"name" description:"新标签名称"`
	}
}

// TAUpdateTagOutput 修改标签
type TAUpdateTagOutput struct {
	Body utils.HumaResponse[any]
}

// TAUpdateTag 重命名标签
func (tc *TagController) TAUpdateTag(ctx context.Context, input *TAUpdateTagInput) (*TAUpdateTagOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的标签ID")
	}
	if input.Body.Name == "" {
		return nil, utils.HumaBadRequest("标签名称不能为空")
	}

	if err := tc.tagService.RenameTag(input.ID, input.Body.Name); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &TAUpdateTagOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "更新成功",
		},
	}, nil
}

// TADeleteTagInput 删除标签
type TADeleteTagInput struct {
	ID string `path:"id" description:"标签ID"`
}

// TADeleteTagOutput 删除标签
type TADeleteTagOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteTag 删除标签
func (tc *TagController) TADeleteTag(ctx context.Context, input *TADeleteTagInput) (*TADeleteTagOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的标签ID")
	}

	if err := tc.tagService.DeleteTag(input.ID); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &TADeleteTagOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// RegisterAPITagRoutes 注册 /api/v1 标签管理 Huma 路由
func (tc *TagController) RegisterAPITagRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tags",
		OperationID: "apiGetTags",
		Summary:     "获取标签列表",
		Description: "获取标签列表，支持分页、模糊搜索、类型过滤以及统计关联的实体数",
		Tags:        []string{"标签管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAGetTags)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tags",
		OperationID: "apiCreateTag",
		Summary:     "新建标签",
		Description: "手动新建标签",
		Tags:        []string{"标签管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TACreateTag)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/tags/{id}",
		OperationID: "apiUpdateTag",
		Summary:     "修改标签",
		Description: "重命名标签名称",
		Tags:        []string{"标签管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAUpdateTag)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/tags/{id}",
		OperationID: "apiDeleteTag",
		Summary:     "删除标签",
		Description: "删除标签，并清理其所有对应绑定关系",
		Tags:        []string{"标签管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TADeleteTag)
}

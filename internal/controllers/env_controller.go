package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/relation"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type EnvController struct {
	envService *services.EnvService
}

func NewEnvController(envService *services.EnvService) *EnvController {
	return &EnvController{envService: envService}
}

// ===========================================================================
// 环境变量业务方法
// ===========================================================================

// GetSecretStatus 获取加密秘钥状态
func (ec *EnvController) GetSecretStatus(ctx context.Context, input *struct{}) (*struct {
	Body utils.HumaResponse[bool]
}, error) {
	return &struct {
		Body utils.HumaResponse[bool]
	}{
		Body: utils.HumaResponse[bool]{
			Code: 200,
			Msg:  "success",
			Data: utils.IsSecretKeySet(),
		},
	}, nil
}

// GetEnvTagsInput 获取所有环境变量标签
type GetEnvTagsOutput struct {
	Body utils.HumaResponse[[]string]
}

// GetEnvTags 获取所有环境变量标签
func (ec *EnvController) GetEnvTags(ctx context.Context, input *struct{}) (*GetEnvTagsOutput, error) {
	tags, err := ec.envService.GetAllEnvTags()
	if err != nil {
		return nil, utils.HumaServerError("获取标签失败")
	}

	return &GetEnvTagsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: tags,
		},
	}, nil
}

// CreateEnvVarInput 创建环境变量
type CreateEnvVarInput struct {
	Body struct {
		Name    string `json:"name" description:"变量名"`
		Value   string `json:"value" description:"变量值"`
		Remark  string `json:"remark" description:"备注"`
		Type    string `json:"type" description:"变量类型"`
		Hidden  *bool  `json:"hidden" description:"是否隐藏"`
		Enabled *bool  `json:"enabled" description:"是否启用"`
		Tags    string `json:"tags" description:"标签"`
	}
}

// CreateEnvVarOutput 创建环境变量
type CreateEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// CreateEnvVar 创建环境变量
func (ec *EnvController) CreateEnvVar(ctx context.Context, input *CreateEnvVarInput) (*CreateEnvVarOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	req := input.Body
	if req.Name == "" || req.Value == "" {
		return nil, utils.HumaBadRequest("变量名和值不能为空")
	}

	if req.Type == "" {
		req.Type = constant.EnvTypeNormal
	}

	hidden := true
	if req.Hidden != nil {
		hidden = *req.Hidden
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	envVar := ec.envService.CreateEnvVar(req.Name, req.Value, req.Remark, req.Type, hidden, enabled, userID)
	if envVar != nil {
		relation.DataRelation.SaveTags(envVar.ID, constant.RelationTypeEnvTag, req.Tags)
		envVar.Tags = req.Tags
	}

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &CreateEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// GetEnvVarsInput 获取环境变量列表
type GetEnvVarsInput struct {
	Name     string `query:"name" description:"按名称模糊查询"`
	Type     string `query:"type" description:"按类型筛选"`
	Tags     string `query:"tags" description:"按标签筛选"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// GetEnvVarsOutput 获取环境变量列表
type GetEnvVarsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.EnvVO]]
}

// GetEnvVars 获取环境变量列表
func (ec *EnvController) GetEnvVars(ctx context.Context, input *GetEnvVarsInput) (*GetEnvVarsOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	envVars, total := ec.envService.GetEnvVarsWithPagination(userID, input.Name, input.Type, input.Tags, page, pageSize)

	return &GetEnvVarsOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]*vo.EnvVO]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]*vo.EnvVO]{
				Data:     vo.ToEnvVOListFromModels(envVars),
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// GetAllEnvVarsOutput 获取所有环境变量
type GetAllEnvVarsOutput struct {
	Body utils.HumaResponse[[]*vo.EnvVO]
}

// GetAllEnvVars 获取所有环境变量
func (ec *EnvController) GetAllEnvVars(ctx context.Context, input *struct{}) (*GetAllEnvVarsOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	envVars := ec.envService.GetEnvVarsByUserID(userID)

	return &GetAllEnvVarsOutput{
		Body: utils.HumaResponse[[]*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVOListFromModels(envVars),
		},
	}, nil
}

// GetEnvVarInput 获取环境变量详情
type GetEnvVarInput struct {
	ID string `path:"id" description:"环境变量ID"`
}

// GetEnvVarOutput 获取环境变量详情
type GetEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// GetEnvVar 获取环境变量详情
func (ec *EnvController) GetEnvVar(ctx context.Context, input *GetEnvVarInput) (*GetEnvVarOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	envVar := ec.envService.GetEnvVarByID(input.ID)
	if envVar == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	return &GetEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// GetAssociatedTasksInput 获取关联任务
type GetAssociatedTasksInput struct {
	ID string `path:"id" description:"环境变量ID"`
}

// GetAssociatedTasksOutput 获取关联任务
type GetAssociatedTasksOutput struct {
	Body utils.HumaResponse[[]*vo.TaskVO]
}

// GetAssociatedTasks 获取关联任务
func (ec *EnvController) GetAssociatedTasks(ctx context.Context, input *GetAssociatedTasksInput) (*GetAssociatedTasksOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	tasks := ec.envService.GetAssociatedTasks(input.ID)

	return &GetAssociatedTasksOutput{
		Body: utils.HumaResponse[[]*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVOListFromModels(tasks),
		},
	}, nil
}

// UpdateEnvVarInput 更新环境变量
type UpdateEnvVarInput struct {
	ID   string `path:"id" description:"环境变量ID"`
	Body struct {
		Name    string `json:"name" description:"变量名"`
		Value   string `json:"value" description:"变量值"`
		Remark  string `json:"remark" description:"备注"`
		Type    string `json:"type" description:"变量类型"`
		Hidden  *bool  `json:"hidden" description:"是否隐藏"`
		Enabled *bool  `json:"enabled" description:"是否启用"`
		Tags    string `json:"tags" description:"标签"`
	}
}

// UpdateEnvVarOutput 更新环境变量
type UpdateEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// UpdateEnvVar 更新环境变量
func (ec *EnvController) UpdateEnvVar(ctx context.Context, input *UpdateEnvVarInput) (*UpdateEnvVarOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	req := input.Body
	if req.Type == "" {
		req.Type = constant.EnvTypeNormal
	}

	// 对于更新，获取现有数据
	existing := ec.envService.GetEnvVarByID(input.ID)
	if existing == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	hidden := existing.Hidden
	if req.Hidden != nil {
		hidden = req.Hidden
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = req.Enabled
	}

	envVar := ec.envService.UpdateEnvVar(input.ID, req.Name, req.Value, req.Remark, req.Type, utils.DerefBool(hidden, true), utils.DerefBool(enabled, true))
	if envVar == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	relation.DataRelation.SaveTags(envVar.ID, constant.RelationTypeEnvTag, req.Tags)
	envVar.Tags = req.Tags

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &UpdateEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// DeleteEnvVarInput 删除环境变量
type DeleteEnvVarInput struct {
	ID    string `path:"id" description:"环境变量ID"`
	Force bool   `query:"force" description:"是否强制删除"`
}

// DeleteEnvVarOutput 删除环境变量
type DeleteEnvVarOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteEnvVar 删除环境变量
func (ec *EnvController) DeleteEnvVar(ctx context.Context, input *DeleteEnvVarInput) (*DeleteEnvVarOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	force := input.Force
	success, associatedTasks := ec.envService.DeleteEnvVar(input.ID, force)

	if len(associatedTasks) > 0 {
		// 业务层面 409 冲突，携带关联任务数据
		return &DeleteEnvVarOutput{
			Body: utils.HumaResponse[any]{
				Code: 409,
				Msg:  "该环境变量已被任务引用，请先在任务中删除引用或选择强制删除",
				Data: vo.ToTaskVOListFromModels(associatedTasks),
			},
		}, nil
	}

	if !success {
		return nil, utils.HumaNotFound("环境变量不存在或删除失败")
	}

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &DeleteEnvVarOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// BulkSaveEnvInput 批量保存环境变量
type BulkSaveEnvInput struct {
	Body []struct {
		ID      string `json:"id" description:"变量ID"`
		Name    string `json:"name" description:"变量名"`
		Value   string `json:"value" description:"变量值"`
		Remark  string `json:"remark" description:"备注"`
		Type    string `json:"type" description:"变量类型"`
		Hidden  *bool  `json:"hidden" description:"是否隐藏"`
		Enabled *bool  `json:"enabled" description:"是否启用"`
	}
}

// BulkSaveEnvOutput 批量保存环境变量
type BulkSaveEnvOutput struct {
	Body utils.HumaResponse[any]
}

// BulkSaveEnv 批量保存环境变量
func (ec *EnvController) BulkSaveEnv(ctx context.Context, input *BulkSaveEnvInput) (*BulkSaveEnvOutput, error) {
	reqs := input.Body
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	for _, req := range reqs {
		if req.Type == constant.EnvTypeSecret {
			continue // 二次严苛拦截，机密变量不应下发/保存
		}

		hidden := true
		if req.Hidden != nil {
			hidden = *req.Hidden
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		var existingEnv *models.EnvironmentVariable
		// 优先按 ID 匹配
		if req.ID != "" {
			var e models.EnvironmentVariable
			if err := database.DB.Where("id = ?", req.ID).First(&e).Error; err == nil {
				existingEnv = &e
			}
		}
		// 如果 ID 没找到，按 Name 匹配
		if existingEnv == nil {
			var e models.EnvironmentVariable
			if err := database.DB.Where("name = ?", req.Name).First(&e).Error; err == nil {
				existingEnv = &e
			}
		}

		if existingEnv != nil {
			existingEnv.Name = req.Name
			existingEnv.Value = models.BigText(req.Value)
			existingEnv.Remark = req.Remark
			existingEnv.Type = req.Type
			existingEnv.Hidden = &hidden
			existingEnv.Enabled = &enabled
			database.DB.Save(existingEnv)

			if req.ID != "" && existingEnv.ID != req.ID {
				database.DB.Model(existingEnv).Update("id", req.ID)
			}
		} else {
			envVar := &models.EnvironmentVariable{
				ID:        req.ID,
				Name:      req.Name,
				Value:     models.BigText(req.Value),
				Remark:    req.Remark,
				Type:      req.Type,
				Hidden:    &hidden,
				Enabled:   &enabled,
				UserID:    userID,
				CreatedAt: models.Now(),
				UpdatedAt: models.Now(),
			}
			if envVar.ID == "" {
				envVar.ID = utils.GenerateID()
			}
			database.DB.Create(envVar)
		}
	}

	services.GetAgentWSManager().BroadcastTasksToAll()

	return &BulkSaveEnvOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// registerEnvRoutes 注册环境变量共用路由（7 条）。
// security 直接作为 OpenAPI 文档的 Security 声明。
// 独有路由（secret-status/tags/bulk_save）仅在 /api/v1 注册，
// 由 RegisterAPIEnvRoutes 单独处理。
func (ec *EnvController) registerEnvRoutes(api huma.API, security []map[string][]string) {
	tag := []string{"环境变量"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/env", OperationID: "CreateEnvVar", Summary: "创建环境变量", Description: "创建一个新的环境变量", Tags: tag, Security: security}, ec.CreateEnvVar)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/env", OperationID: "GetEnvVars", Summary: "获取环境变量列表", Description: "分页获取环境变量列表，支持名称、类型、标签筛选", Tags: tag, Security: security}, ec.GetEnvVars)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/env/all", OperationID: "GetAllEnvVars", Summary: "获取所有环境变量", Description: "获取当前用户的所有环境变量", Tags: tag, Security: security}, ec.GetAllEnvVars)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/env/{id}", OperationID: "GetEnvVar", Summary: "获取环境变量详情", Description: "根据 ID 获取环境变量详情", Tags: tag, Security: security}, ec.GetEnvVar)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/env/{id}/tasks", OperationID: "GetAssociatedTasks", Summary: "获取关联任务", Description: "获取引用该环境变量的任务列表", Tags: tag, Security: security}, ec.GetAssociatedTasks)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/env/{id}", OperationID: "UpdateEnvVar", Summary: "更新环境变量", Description: "根据 ID 更新环境变量", Tags: tag, Security: security}, ec.UpdateEnvVar)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/env/{id}", OperationID: "DeleteEnvVar", Summary: "删除环境变量", Description: "根据 ID 删除环境变量，若被任务引用则返回 409", Tags: tag, Security: security}, ec.DeleteEnvVar)
}

// RegisterAPIEnvRoutes 注册 /api/v1 环境变量 Huma 路由（CookieAuth）
// 含独有接口：secret-status、tags、bulk_save
func (ec *EnvController) RegisterAPIEnvRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"环境变量"}

	// 独有接口
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/env/secret-status", OperationID: "GetSecretStatus", Summary: "获取加密秘钥状态", Description: "返回系统是否已配置加密秘钥", Tags: tag, Security: security}, ec.GetSecretStatus)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/env/tags", OperationID: "GetEnvTags", Summary: "获取所有环境变量标签", Description: "获取所有环境变量中使用的标签列表", Tags: tag, Security: security}, ec.GetEnvTags)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/env/bulk_save", OperationID: "BulkSaveEnv", Summary: "批量保存环境变量", Description: "批量保存环境变量，支持新建与更新", Tags: tag, Security: security}, ec.BulkSaveEnv)

	// 共用接口
	ec.registerEnvRoutes(api, security)
}

// RegisterOpenAPIEnvRoutes 注册 OpenAPI 环境变量 Huma 路由（BearerAuth，子集）
func (ec *EnvController) RegisterOpenAPIEnvRoutes(api huma.API) {
	ec.registerEnvRoutes(api, []map[string][]string{{"BearerAuth": {}}})
}

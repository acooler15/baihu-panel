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

// ===========================================================================
// /api/v1 管理接口 —— 环境变量
// ===========================================================================

// TAGetSecretStatusOutput 获取加密秘钥状态
type TAGetSecretStatusOutput struct {
	Body utils.HumaResponse[bool]
}

// TAGetSecretStatus 获取加密秘钥状态
func (ec *EnvController) TAGetSecretStatus(ctx context.Context, input *struct{}) (*TAGetSecretStatusOutput, error) {
	return &TAGetSecretStatusOutput{
		Body: utils.HumaResponse[bool]{
			Code: 200,
			Msg:  "success",
			Data: utils.IsSecretKeySet(),
		},
	}, nil
}

// TAGetEnvTagsOutput 获取所有环境变量标签
type TAGetEnvTagsOutput struct {
	Body utils.HumaResponse[[]string]
}

// TAGetEnvTags 获取所有环境变量标签
func (ec *EnvController) TAGetEnvTags(ctx context.Context, input *struct{}) (*TAGetEnvTagsOutput, error) {
	tags, err := ec.envService.GetAllEnvTags()
	if err != nil {
		return nil, utils.HumaServerError("获取标签失败")
	}

	return &TAGetEnvTagsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: tags,
		},
	}, nil
}

// TACreateEnvVarInput 创建环境变量
type TACreateEnvVarInput struct {
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

// TACreateEnvVarOutput 创建环境变量
type TACreateEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// TACreateEnvVar 创建环境变量
func (ec *EnvController) TACreateEnvVar(ctx context.Context, input *TACreateEnvVarInput) (*TACreateEnvVarOutput, error) {
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

	return &TACreateEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// TAGetEnvVarsInput 获取环境变量列表
type TAGetEnvVarsInput struct {
	Name     string `query:"name" description:"按名称模糊查询"`
	Type     string `query:"type" description:"按类型筛选"`
	Tags     string `query:"tags" description:"按标签筛选"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// TAGetEnvVarsOutput 获取环境变量列表
type TAGetEnvVarsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.EnvVO]]
}

// TAGetEnvVars 获取环境变量列表
func (ec *EnvController) TAGetEnvVars(ctx context.Context, input *TAGetEnvVarsInput) (*TAGetEnvVarsOutput, error) {
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

	return &TAGetEnvVarsOutput{
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

// TAGetAllEnvVarsOutput 获取所有环境变量
type TAGetAllEnvVarsOutput struct {
	Body utils.HumaResponse[[]*vo.EnvVO]
}

// TAGetAllEnvVars 获取所有环境变量
func (ec *EnvController) TAGetAllEnvVars(ctx context.Context, input *struct{}) (*TAGetAllEnvVarsOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	envVars := ec.envService.GetEnvVarsByUserID(userID)

	return &TAGetAllEnvVarsOutput{
		Body: utils.HumaResponse[[]*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVOListFromModels(envVars),
		},
	}, nil
}

// TAGetEnvVarInput 获取环境变量详情
type TAGetEnvVarInput struct {
	ID string `path:"id" description:"环境变量ID"`
}

// TAGetEnvVarOutput 获取环境变量详情
type TAGetEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// TAGetEnvVar 获取环境变量详情
func (ec *EnvController) TAGetEnvVar(ctx context.Context, input *TAGetEnvVarInput) (*TAGetEnvVarOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	envVar := ec.envService.GetEnvVarByID(input.ID)
	if envVar == nil {
		return nil, utils.HumaNotFound("环境变量不存在")
	}

	return &TAGetEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// TAGetAssociatedTasksInput 获取关联任务
type TAGetAssociatedTasksInput struct {
	ID string `path:"id" description:"环境变量ID"`
}

// TAGetAssociatedTasksOutput 获取关联任务
type TAGetAssociatedTasksOutput struct {
	Body utils.HumaResponse[[]*vo.TaskVO]
}

// TAGetAssociatedTasks 获取关联任务
func (ec *EnvController) TAGetAssociatedTasks(ctx context.Context, input *TAGetAssociatedTasksInput) (*TAGetAssociatedTasksOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	tasks := ec.envService.GetAssociatedTasks(input.ID)

	return &TAGetAssociatedTasksOutput{
		Body: utils.HumaResponse[[]*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVOListFromModels(tasks),
		},
	}, nil
}

// TAUpdateEnvVarInput 更新环境变量
type TAUpdateEnvVarInput struct {
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

// TAUpdateEnvVarOutput 更新环境变量
type TAUpdateEnvVarOutput struct {
	Body utils.HumaResponse[*vo.EnvVO]
}

// TAUpdateEnvVar 更新环境变量
func (ec *EnvController) TAUpdateEnvVar(ctx context.Context, input *TAUpdateEnvVarInput) (*TAUpdateEnvVarOutput, error) {
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

	return &TAUpdateEnvVarOutput{
		Body: utils.HumaResponse[*vo.EnvVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToEnvVO(envVar),
		},
	}, nil
}

// TADeleteEnvVarInput 删除环境变量
type TADeleteEnvVarInput struct {
	ID    string `path:"id" description:"环境变量ID"`
	Force bool   `query:"force" description:"是否强制删除"`
}

// TADeleteEnvVarOutput 删除环境变量
type TADeleteEnvVarOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteEnvVar 删除环境变量
func (ec *EnvController) TADeleteEnvVar(ctx context.Context, input *TADeleteEnvVarInput) (*TADeleteEnvVarOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的环境变量ID")
	}

	force := input.Force
	success, associatedTasks := ec.envService.DeleteEnvVar(input.ID, force)

	if len(associatedTasks) > 0 {
		// 业务层面 409 冲突，携带关联任务数据
		c := utils.GetGinContext(ctx)
		if c != nil {
			c.JSON(http.StatusOK, utils.Response{
				Code: 409,
				Msg:  "该环境变量已被任务引用，请先在任务中删除引用或选择强制删除",
				Data: vo.ToTaskVOListFromModels(associatedTasks),
			})
			c.Abort()
		}
		return &TADeleteEnvVarOutput{
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

	return &TADeleteEnvVarOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TABulkSaveEnvInput 批量保存环境变量
type TABulkSaveEnvInput struct {
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

// TABulkSaveEnvOutput 批量保存环境变量
type TABulkSaveEnvOutput struct {
	Body utils.HumaResponse[any]
}

// TABulkSaveEnv 批量保存环境变量
func (ec *EnvController) TABulkSaveEnv(ctx context.Context, input *TABulkSaveEnvInput) (*TABulkSaveEnvOutput, error) {
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

	return &TABulkSaveEnvOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// RegisterAPIEnvRoutes 注册 /api/v1 环境变量 Huma 路由
func (ec *EnvController) RegisterAPIEnvRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/secret-status",
		OperationID: "apiGetSecretStatus",
		Summary:     "获取加密秘钥状态",
		Description: "返回系统是否已配置加密秘钥",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetSecretStatus)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/tags",
		OperationID: "apiGetEnvTags",
		Summary:     "获取所有环境变量标签",
		Description: "获取所有环境变量中使用的标签列表",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetEnvTags)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/env",
		OperationID: "apiCreateEnvVar",
		Summary:     "创建环境变量",
		Description: "创建一个新的环境变量",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TACreateEnvVar)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/env/bulk_save",
		OperationID: "apiBulkSaveEnv",
		Summary:     "批量保存环境变量",
		Description: "批量保存环境变量，支持新建与更新",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TABulkSaveEnv)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env",
		OperationID: "apiGetEnvVars",
		Summary:     "获取环境变量列表",
		Description: "分页获取环境变量列表，支持名称、类型、标签筛选",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetEnvVars)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/all",
		OperationID: "apiGetAllEnvVars",
		Summary:     "获取所有环境变量",
		Description: "获取当前用户的所有环境变量",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetAllEnvVars)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/{id}",
		OperationID: "apiGetEnvVar",
		Summary:     "获取环境变量详情",
		Description: "根据 ID 获取环境变量详情",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetEnvVar)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/env/{id}/tasks",
		OperationID: "apiGetAssociatedTasks",
		Summary:     "获取关联任务",
		Description: "获取引用该环境变量的任务列表",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetAssociatedTasks)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/env/{id}",
		OperationID: "apiUpdateEnvVar",
		Summary:     "更新环境变量",
		Description: "根据 ID 更新环境变量",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAUpdateEnvVar)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/env/{id}",
		OperationID: "apiDeleteEnvVar",
		Summary:     "删除环境变量",
		Description: "根据 ID 删除环境变量，若被任务引用则返回 409",
		Tags:        []string{"环境变量"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TADeleteEnvVar)
}

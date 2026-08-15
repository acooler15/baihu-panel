package controllers

import (
	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/relation"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type EnvController struct {
	envService *services.EnvService
}

func NewEnvController(envService *services.EnvService) *EnvController {
	return &EnvController{envService: envService}
}

// GetSecretStatus 获取加密秘钥状态
// 已迁移至 Huma：TAGetSecretStatus /api/v1/env/secret-status
func (ec *EnvController) GetSecretStatus(c *gin.Context) {
	utils.Success(c, utils.IsSecretKeySet())
}

// CreateEnvVar 创建环境变量
func (ec *EnvController) CreateEnvVar(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		Name    string `json:"name" binding:"required"`
		Value   string `json:"value" binding:"required"`
		Remark  string `json:"remark"`
		Type    string `json:"type"`
		Hidden  *bool  `json:"hidden"`
		Enabled *bool  `json:"enabled"`
		Tags    string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
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
	
	// Broadcast tasks to all agents because global envs changed
	services.GetAgentWSManager().BroadcastTasksToAll()
	
	utils.Success(c, vo.ToEnvVO(envVar))
}

// GetEnvVars 获取环境变量列表
func (ec *EnvController) GetEnvVars(c *gin.Context) {
	userID := c.GetString("userID")
	p := utils.ParsePagination(c)
	name := c.DefaultQuery("name", "")
	envType := c.DefaultQuery("type", "")
	tags := c.DefaultQuery("tags", "")
	envVars, total := ec.envService.GetEnvVarsWithPagination(userID, name, envType, tags, p.Page, p.PageSize)
	utils.PaginatedResponse(c, vo.ToEnvVOListFromModels(envVars), total, p)
}

// GetAllEnvVars 获取所有环境变量
func (ec *EnvController) GetAllEnvVars(c *gin.Context) {
	userID := c.GetString("userID")
	envVars := ec.envService.GetEnvVarsByUserID(userID)
	utils.Success(c, vo.ToEnvVOListFromModels(envVars))
}

// GetEnvVar 获取环境变量详情
func (ec *EnvController) GetEnvVar(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的环境变量ID")
		return
	}

	envVar := ec.envService.GetEnvVarByID(id)
	if envVar == nil {
		utils.NotFound(c, "环境变量不存在")
		return
	}

	utils.Success(c, vo.ToEnvVO(envVar))
}

// UpdateEnvVar 更新环境变量
func (ec *EnvController) UpdateEnvVar(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的环境变量ID")
		return
	}

	var req struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		Remark  string `json:"remark"`
		Type    string `json:"type"`
		Hidden  *bool  `json:"hidden"`
		Enabled *bool  `json:"enabled"`
		Tags    string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if req.Type == "" {
		req.Type = constant.EnvTypeNormal
	}

	// 对于更新，获取现有数据
	existing := ec.envService.GetEnvVarByID(id)
	if existing == nil {
		utils.NotFound(c, "环境变量不存在")
		return
	}

	hidden := existing.Hidden
	if req.Hidden != nil {
		hidden = req.Hidden
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = req.Enabled
	}

	envVar := ec.envService.UpdateEnvVar(id, req.Name, req.Value, req.Remark, req.Type, utils.DerefBool(hidden, true), utils.DerefBool(enabled, true))
	if envVar == nil {
		utils.NotFound(c, "环境变量不存在")
		return
	}

	relation.DataRelation.SaveTags(envVar.ID, constant.RelationTypeEnvTag, req.Tags)
	envVar.Tags = req.Tags

	// Broadcast tasks to all agents because global envs changed
	services.GetAgentWSManager().BroadcastTasksToAll()

	utils.Success(c, vo.ToEnvVO(envVar))
}

// DeleteEnvVar 删除环境变量
func (ec *EnvController) DeleteEnvVar(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的环境变量ID")
		return
	}

	force := c.Query("force") == "true"
	success, associatedTasks := ec.envService.DeleteEnvVar(id, force)

	if len(associatedTasks) > 0 {
		c.JSON(200, utils.Response{
			Code: 409,
			Msg:  "该环境变量已被任务引用，请先在任务中删除引用或选择强制删除",
			Data: vo.ToTaskVOListFromModels(associatedTasks),
		})
		return
	}

	if !success {
		utils.NotFound(c, "环境变量不存在或删除失败")
		return
	}

	// Broadcast tasks to all agents because global envs changed
	services.GetAgentWSManager().BroadcastTasksToAll()

	utils.SuccessMsg(c, "删除成功")
}

// GetAssociatedTasks 获取关联任务
func (ec *EnvController) GetAssociatedTasks(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的环境变量ID")
		return
	}
	tasks := ec.envService.GetAssociatedTasks(id)
	utils.Success(c, vo.ToTaskVOListFromModels(tasks))
}

// GetTags 获取所有环境变量标签
// 已迁移至 Huma：TAGetEnvTags /api/v1/env/tags
func (ec *EnvController) GetTags(c *gin.Context) {
	tags, err := ec.envService.GetAllEnvTags()
	if err != nil {
		utils.ServerError(c, "获取标签失败")
		return
	}
	utils.Success(c, tags)
}

// BulkSaveEnv 批量保存环境变量
func (ec *EnvController) BulkSaveEnv(c *gin.Context) {
	var reqs []struct {
		ID      string `json:"id"`
		Name    string `json:"name" binding:"required"`
		Value   string `json:"value" binding:"required"`
		Remark  string `json:"remark"`
		Type    string `json:"type"`
		Hidden  *bool  `json:"hidden"`
		Enabled *bool  `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&reqs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID := c.GetString("userID")

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
	utils.Success(c, nil)
}

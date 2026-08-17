package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type DataController struct {
	dataService    *services.DataService
	taskController *TaskController
	envController  *EnvController
}

func NewDataController(tc *TaskController, ec *EnvController) *DataController {
	return &DataController{
		dataService:    services.NewDataService(),
		taskController: tc,
		envController:  ec,
	}
}

// ===========================================================================
// 数据导入导出业务方法
// ===========================================================================

// ExportBusinessDataInput 导出业务数据
type ExportBusinessDataInput struct {
	Body struct {
		TaskIDs []string `json:"task_ids" description:"任务 ID 列表"`
		EnvIDs  []string `json:"env_ids" description:"环境变量 ID 列表"`
	}
}

// ExportBusinessDataOutput 导出业务数据
type ExportBusinessDataOutput struct {
	Body utils.HumaResponse[*models.ExportData]
}

// ExportBusinessData 导出业务数据
func (dc *DataController) ExportBusinessData(ctx context.Context, input *ExportBusinessDataInput) (*ExportBusinessDataOutput, error) {
	exportData := dc.dataService.ExportBusinessData(input.Body.TaskIDs, input.Body.EnvIDs)

	return &ExportBusinessDataOutput{
		Body: utils.HumaResponse[*models.ExportData]{
			Code: 200,
			Msg:  "success",
			Data: exportData,
		},
	}, nil
}

// ImportBusinessDataInput 导入业务数据
type ImportBusinessDataInput struct {
	Body models.ExportData
}

// ImportBusinessDataOutput 导入业务数据
type ImportBusinessDataOutput struct {
	Body utils.HumaResponse[any]
}

// ImportBusinessData 导入业务数据
func (dc *DataController) ImportBusinessData(ctx context.Context, input *ImportBusinessDataInput) (*ImportBusinessDataOutput, error) {
	req := input.Body

	if req.Version == "" {
		return nil, utils.HumaBadRequest("无效的导入数据格式")
	}

	// 停止相关的定时任务
	if len(req.Tasks) > 0 {
		for _, task := range req.Tasks {
			dc.taskController.executorService.RemoveCronTask(task.ID)
			dc.taskController.executorService.GetScheduler().StopTask(task.ID)
		}
	}

	// 导入数据
	if err := dc.dataService.ImportBusinessData(&req); err != nil {
		return nil, utils.HumaServerError("导入失败: " + err.Error())
	}

	// 重新启动任务和通知相关的代理
	if len(req.Tasks) > 0 {
		for i := range req.Tasks {
			task := &req.Tasks[i]
			if utils.DerefBool(task.Enabled, true) && (task.AgentID == nil || *task.AgentID == "") {
				dc.taskController.executorService.AddCronTask(task)
			}
			if task.AgentID != nil && *task.AgentID != "" {
				dc.taskController.agentWSManager.BroadcastTasks(*task.AgentID)
			}
		}
	}

	return &ImportBusinessDataOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "导入成功",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIDataRoutes 注册 /api/v1 数据导入导出 Huma 路由
func (dc *DataController) RegisterAPIDataRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"数据管理"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/system/export", OperationID: "ExportBusinessData", Summary: "导出业务数据", Description: "导出任务、环境变量等业务数据", Tags: tag, Security: security}, dc.ExportBusinessData)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/system/import", OperationID: "ImportBusinessData", Summary: "导入业务数据", Description: "导入任务、环境变量等业务数据", Tags: tag, Security: security}, dc.ImportBusinessData)
}

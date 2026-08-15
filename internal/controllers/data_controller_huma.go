package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 数据导入导出
// ===========================================================================

// TAExportBusinessDataInput 导出业务数据
type TAExportBusinessDataInput struct {
	Body struct {
		TaskIDs []string `json:"task_ids" description:"任务 ID 列表"`
		EnvIDs  []string `json:"env_ids" description:"环境变量 ID 列表"`
	}
}

// TAExportBusinessDataOutput 导出业务数据
type TAExportBusinessDataOutput struct {
	Body utils.HumaResponse[*models.ExportData]
}

// TAExportBusinessData 导出业务数据
func (dc *DataController) TAExportBusinessData(ctx context.Context, input *TAExportBusinessDataInput) (*TAExportBusinessDataOutput, error) {
	exportData := dc.dataService.ExportBusinessData(input.Body.TaskIDs, input.Body.EnvIDs)

	return &TAExportBusinessDataOutput{
		Body: utils.HumaResponse[*models.ExportData]{
			Code: 200,
			Msg:  "success",
			Data: exportData,
		},
	}, nil
}

// TAImportBusinessDataInput 导入业务数据
type TAImportBusinessDataInput struct {
	Body models.ExportData
}

// TAImportBusinessDataOutput 导入业务数据
type TAImportBusinessDataOutput struct {
	Body utils.HumaResponse[any]
}

// TAImportBusinessData 导入业务数据
func (dc *DataController) TAImportBusinessData(ctx context.Context, input *TAImportBusinessDataInput) (*TAImportBusinessDataOutput, error) {
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

	return &TAImportBusinessDataOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "导入成功",
		},
	}, nil
}

// RegisterAPIDataRoutes 注册 /api/v1 数据导入导出 Huma 路由
func (dc *DataController) RegisterAPIDataRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/system/export",
		OperationID: "apiExportBusinessData",
		Summary:     "导出业务数据",
		Description: "导出任务、环境变量等业务数据",
		Tags:        []string{"数据管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.TAExportBusinessData)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/system/import",
		OperationID: "apiImportBusinessData",
		Summary:     "导入业务数据",
		Description: "导入任务、环境变量等业务数据",
		Tags:        []string{"数据管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, dc.TAImportBusinessData)
}

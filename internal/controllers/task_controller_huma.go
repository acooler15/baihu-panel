package controllers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
	"os"
)

// ===========================================================================
// OpenAPI (Bearer Token) 接口 —— 任务管理
// ===========================================================================

// OACreateTaskInput 创建任务（OpenAPI）
type OACreateTaskInput struct {
	Body vo.TaskCreateReq
}

// OACreateTaskOutput 创建任务（OpenAPI）
type OACreateTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// OACreateTask 创建任务
func (tc *TaskController) OACreateTask(ctx context.Context, input *OACreateTaskInput) (*OACreateTaskOutput, error) {
	req := input.Body

	// 普通任务需要命令
	if req.Type != constant.TaskTypeRepo && req.Command == "" {
		return nil, utils.HumaBadRequest("命令不能为空")
	}

	if req.Schedule != "" {
		if err := tc.executorService.ValidateCron(req.Schedule); err != nil {
			return nil, utils.HumaBadRequest("无效的cron表达式: " + err.Error())
		}
	}

	// 转换为绝对路径（Agent 任务保持原样）
	workDir := req.WorkDir
	if req.AgentID == nil || *req.AgentID == "" {
		workDir = resolveWorkDir(req.WorkDir)
	}

	var sourceID string
	// 如果是仓库同步任务，根据 URL 生成 SourceID 用于去重
	if req.Type == constant.TaskTypeRepo && req.Config != "" {
		var repoCfg struct {
			SourceURL   string `json:"source_url"`
			Branch      string `json:"branch"`
			RepoDirName string `json:"repo_dir_name"`
			TargetPath  string `json:"target_path"`
		}
		if err := json.Unmarshal([]byte(req.Config), &repoCfg); err == nil && repoCfg.SourceURL != "" {
			if repoCfg.RepoDirName != "" {
				if !isValidDirName(repoCfg.RepoDirName) {
					return nil, utils.HumaBadRequest("自定义目录名只能包含字母、数字、下划线、短划线和点，不能只有点，且不能包含路径逻辑")
				}
			}

			if repoCfg.RepoDirName != "" {
				sourceID = "repo_" + repoCfg.RepoDirName
			} else {
				sourceID = "repo_" + utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
			}

			existingTask := tc.taskService.GetTaskBySourceID(sourceID)
			if existingTask != nil {
				return nil, utils.HumaBadRequest("当前任务已存在，请检查或更换仓库目录名称")
			}

			newAbsPath := getRepoPhysicalPath(repoCfg.TargetPath, repoCfg.RepoDirName, repoCfg.SourceURL, repoCfg.Branch)
			if newAbsPath != "" {
				if info, err := os.Stat(newAbsPath); err == nil && info.IsDir() {
					return nil, utils.HumaBadRequest("本地已存在同名仓库文件夹，请更换自定义目录名或清理残留文件")
				}
			}
		}
	}

	param := tasks.TaskParam{
		Name:          req.Name,
		Remark:        req.Remark,
		Command:       req.Command,
		PreCommand:    req.PreCommand,
		PostCommand:   req.PostCommand,
		Tags:          req.Tags,
		Type:          req.Type,
		Config:        req.Config,
		Schedule:      req.Schedule,
		Timeout:       req.Timeout,
		WorkDir:       workDir,
		CleanConfig:   req.CleanConfig,
		Envs:          req.Envs,
		Languages:     req.Languages,
		AgentID:       req.AgentID,
		TriggerType:   req.TriggerType,
		RetryCount:    req.RetryCount,
		RetryInterval: req.RetryInterval,
		RandomRange:   req.RandomRange,
		SourceID:      sourceID,
		PinType:       req.PinType,
		Enabled:       true,
	}

	var task = tc.taskService.GetTaskBySourceID(sourceID)
	if task != nil {
		task = tc.taskService.UpdateTask(task.ID, &param)
	}
	if task == nil {
		task = tc.taskService.CreateTask(&param)
	}

	if task.AgentID != nil && *task.AgentID != "" {
		tc.agentWSManager.BroadcastTasks(*task.AgentID)
	} else {
		tc.executorService.AddCronTask(task)
	}

	return &OACreateTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// OAGetTasksInput 获取任务列表（OpenAPI）
type OAGetTasksInput struct {
	Name     string `query:"name" description:"任务名称"`
	AgentID  string `query:"agent_id" description:"Agent ID"`
	Tags     string `query:"tags" description:"标签"`
	Type     string `query:"type" description:"任务类型"`
	SortBy   string `query:"sort_by" description:"排序字段"`
	Order    string `query:"order" description:"排序方向"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// OAGetTasksOutput 获取任务列表（OpenAPI）
type OAGetTasksOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.TaskVO]]
}

// OAGetTasks 获取任务列表
func (tc *TaskController) OAGetTasks(ctx context.Context, input *OAGetTasksInput) (*OAGetTasksOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	var agentID *string
	if input.AgentID != "" {
		id := input.AgentID
		agentID = &id
	}

	taskList, total := tc.taskService.GetTasksWithPagination(page, pageSize, input.Name, agentID, input.Tags, input.Type, input.SortBy, input.Order)

	return &OAGetTasksOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]*vo.TaskVO]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]*vo.TaskVO]{
				Data:     vo.ToTaskVOListFromModels(taskList),
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// OAGetTaskInput 获取任务详情（OpenAPI）
type OAGetTaskInput struct {
	ID string `path:"id" description:"任务ID"`
}

// OAGetTaskOutput 获取任务详情（OpenAPI）
type OAGetTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// OAGetTask 获取任务详情
func (tc *TaskController) OAGetTask(ctx context.Context, input *OAGetTaskInput) (*OAGetTaskOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	return &OAGetTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// OAUpdateTaskInput 更新任务（OpenAPI）
type OAUpdateTaskInput struct {
	ID   string            `path:"id" description:"任务ID"`
	Body vo.TaskUpdateReq
}

// OAUpdateTaskOutput 更新任务（OpenAPI）
type OAUpdateTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// OAUpdateTask 更新任务
func (tc *TaskController) OAUpdateTask(ctx context.Context, input *OAUpdateTaskInput) (*OAUpdateTaskOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	oldTask := tc.taskService.GetTaskByID(id)
	var oldAgentID *string
	if oldTask != nil {
		oldAgentID = oldTask.AgentID
	}

	req := input.Body

	if req.Schedule != "" {
		if err := tc.executorService.ValidateCron(req.Schedule); err != nil {
			return nil, utils.HumaBadRequest("无效的cron表达式: " + err.Error())
		}
	}

	workDir := req.WorkDir
	if req.AgentID == nil || *req.AgentID == "" {
		workDir = resolveWorkDir(req.WorkDir)
	}

	var sourceID string
	if req.Type == constant.TaskTypeRepo && req.Config != "" {
		var repoCfg struct {
			SourceURL   string `json:"source_url"`
			Branch      string `json:"branch"`
			RepoDirName string `json:"repo_dir_name"`
			TargetPath  string `json:"target_path"`
		}
		if err := json.Unmarshal([]byte(req.Config), &repoCfg); err == nil && repoCfg.SourceURL != "" {
			if repoCfg.RepoDirName != "" {
				if !isValidDirName(repoCfg.RepoDirName) {
					return nil, utils.HumaBadRequest("自定义目录名只能包含字母、数字、下划线、短划线和点，不能只有点，且不能包含路径逻辑")
				}
			}

			if repoCfg.RepoDirName != "" {
				sourceID = "repo_" + repoCfg.RepoDirName
			} else {
				sourceID = "repo_" + utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
			}

			if oldTask != nil && sourceID != oldTask.SourceID {
				existingTask := tc.taskService.GetTaskBySourceID(sourceID)
				if existingTask != nil && existingTask.ID != oldTask.ID {
					return nil, utils.HumaBadRequest("当前任务已存在，请检查或更换仓库目录名称")
				}
			}

			newAbsPath := getRepoPhysicalPath(repoCfg.TargetPath, repoCfg.RepoDirName, repoCfg.SourceURL, repoCfg.Branch)

			var oldAbsPath string
			if oldTask != nil && oldTask.Type == constant.TaskTypeRepo && oldTask.Config != "" {
				var oldCfg struct {
					SourceURL   string `json:"source_url"`
					Branch      string `json:"branch"`
					RepoDirName string `json:"repo_dir_name"`
					TargetPath  string `json:"target_path"`
				}
				if json.Unmarshal([]byte(oldTask.Config), &oldCfg) == nil {
					oldAbsPath = getRepoPhysicalPath(oldCfg.TargetPath, oldCfg.RepoDirName, oldCfg.SourceURL, oldCfg.Branch)
				}
			}

			if newAbsPath != "" && newAbsPath != oldAbsPath {
				if info, err := os.Stat(newAbsPath); err == nil && info.IsDir() {
					return nil, utils.HumaBadRequest("目标目录在本地已存在同名文件夹，请更换目录名或清理残留文件")
				}
			}
		}
	} else if oldTask != nil {
		sourceID = oldTask.SourceID
	}

	param := tasks.TaskParam{
		Name:          req.Name,
		Remark:        req.Remark,
		Command:       req.Command,
		PreCommand:    req.PreCommand,
		PostCommand:   req.PostCommand,
		Tags:          req.Tags,
		Type:          req.Type,
		Config:        req.Config,
		Schedule:      req.Schedule,
		Timeout:       req.Timeout,
		WorkDir:       workDir,
		CleanConfig:   req.CleanConfig,
		Envs:          req.Envs,
		Languages:     req.Languages,
		AgentID:       req.AgentID,
		TriggerType:   req.TriggerType,
		RetryCount:    req.RetryCount,
		RetryInterval: req.RetryInterval,
		RandomRange:   req.RandomRange,
		SourceID:      sourceID,
		PinType:       req.PinType,
		Enabled:       req.Enabled,
	}

	task := tc.taskService.UpdateTask(id, &param)
	if task == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	if task.AgentID != nil && *task.AgentID != "" {
		tc.executorService.RemoveCronTask(task.ID)
		tc.agentWSManager.BroadcastTasks(*task.AgentID)
		if oldAgentID != nil && *oldAgentID != "" && *oldAgentID != *task.AgentID {
			tc.agentWSManager.BroadcastTasks(*oldAgentID)
		}
	} else {
		if utils.DerefBool(task.Enabled, true) {
			tc.executorService.AddCronTask(task)
		} else {
			tc.executorService.RemoveCronTask(task.ID)
		}
		if oldAgentID != nil && *oldAgentID != "" {
			tc.agentWSManager.BroadcastTasks(*oldAgentID)
		}
	}

	return &OAUpdateTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// OADeleteTaskInput 删除任务（OpenAPI）
type OADeleteTaskInput struct {
	ID          string `path:"id" description:"任务ID"`
	DeleteFiles bool   `query:"delete_files" description:"是否同时删除物理文件"`
}

// OADeleteTaskOutput 删除任务（OpenAPI）
type OADeleteTaskOutput struct {
	Body utils.HumaResponse[any]
}

// OADeleteTask 删除任务
func (tc *TaskController) OADeleteTask(ctx context.Context, input *OADeleteTaskInput) (*OADeleteTaskOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	agentID := task.AgentID
	deleteFiles := input.DeleteFiles

	if deleteFiles && task.Type == constant.TaskTypeRepo {
		tc.deleteRepoPhysicalFiles(task)
	}

	tc.executorService.RemoveCronTask(id)
	tc.executorService.GetScheduler().StopTask(id)

	success := tc.taskService.DeleteTask(id)
	if !success {
		return nil, utils.HumaNotFound("任务不存在")
	}

	if agentID != nil && *agentID != "" {
		tc.agentWSManager.BroadcastTasks(*agentID)
	}

	return &OADeleteTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// OAStopTaskInput 停止任务（OpenAPI）
type OAStopTaskInput struct {
	LogID string `path:"logID" description:"运行日志ID"`
}

// OAStopTaskOutput 停止任务（OpenAPI）
type OAStopTaskOutput struct {
	Body utils.HumaResponse[any]
}

// OAStopTask 停止任务
func (tc *TaskController) OAStopTask(ctx context.Context, input *OAStopTaskInput) (*OAStopTaskOutput, error) {
	logID := input.LogID
	if logID == "" {
		return nil, utils.HumaBadRequest("无效的日志ID")
	}

	err := tc.executorService.StopTaskExecution(logID)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &OAStopTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "停止请求已发送",
		},
	}, nil
}

// OAGetTagsOutput 获取所有任务标签（OpenAPI）
type OAGetTagsOutput struct {
	Body utils.HumaResponse[[]string]
}

// OAGetTags 获取所有任务标签
func (tc *TaskController) OAGetTags(ctx context.Context, input *struct{}) (*OAGetTagsOutput, error) {
	tags, err := tc.taskService.GetAllTags()
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &OAGetTagsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: tags,
		},
	}, nil
}

// RegisterOpenAPITaskRoutes 注册 OpenAPI 任务相关 Huma 路由
func (tc *TaskController) RegisterOpenAPITaskRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tasks",
		OperationID: "openapiCreateTask",
		Summary:     "创建任务",
		Description: "创建一个新的任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OACreateTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tasks",
		OperationID: "openapiGetTasks",
		Summary:     "获取任务列表",
		Description: "分页获取任务列表，支持按名称、Agent ID、标签、类型筛选",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OAGetTasks)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tasks/{id}",
		OperationID: "openapiGetTask",
		Summary:     "获取任务详情",
		Description: "根据 ID 获取任务详情",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OAGetTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/tasks/{id}",
		OperationID: "openapiUpdateTask",
		Summary:     "更新任务",
		Description: "根据 ID 更新任务信息",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OAUpdateTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}",
		OperationID: "openapiDeleteTask",
		Summary:     "删除任务",
		Description: "根据 ID 删除任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OADeleteTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tasks/stop/{logID}",
		OperationID: "openapiStopTask",
		Summary:     "停止任务",
		Description: "根据运行日志 ID 停止正在执行的任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OAStopTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tasks/tags",
		OperationID: "openapiGetTaskTags",
		Summary:     "获取所有任务标签",
		Description: "获取系统中所有任务已使用的唯一标签列表",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, tc.OAGetTags)
}

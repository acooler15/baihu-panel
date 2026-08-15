package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 任务管理
// ===========================================================================

// TACreateTaskInput 创建任务
type TACreateTaskInput struct {
	Body vo.TaskCreateReq
}

// TACreateTaskOutput 创建任务
type TACreateTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// TACreateTask 创建任务
func (tc *TaskController) TACreateTask(ctx context.Context, input *TACreateTaskInput) (*TACreateTaskOutput, error) {
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

	var task *models.Task
	// 去重逻辑：如果已存在相同 SourceID 的仓库任务，则改为更新
	if sourceID != "" {
		task = tc.taskService.GetTaskBySourceID(sourceID)
		if task != nil {
			task = tc.taskService.UpdateTask(task.ID, &param)
		}
	}

	if task == nil {
		task = tc.taskService.CreateTask(&param)
	}

	// 如果是 Agent 任务，通知 Agent；否则添加到本地 cron
	if task.AgentID != nil && *task.AgentID != "" {
		tc.agentWSManager.BroadcastTasks(*task.AgentID)
	} else {
		tc.executorService.AddCronTask(task)
	}

	return &TACreateTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// TAGetTasksInput 获取任务列表
type TAGetTasksInput struct {
	Name     string `query:"name" description:"任务名称"`
	AgentID  string `query:"agent_id" description:"Agent ID"`
	Tags     string `query:"tags" description:"标签"`
	Type     string `query:"type" description:"任务类型"`
	SortBy   string `query:"sort_by" description:"排序字段"`
	Order    string `query:"order" description:"排序方向"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// TAGetTasksOutput 获取任务列表
type TAGetTasksOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.TaskVO]]
}

// TAGetTasks 获取任务列表
func (tc *TaskController) TAGetTasks(ctx context.Context, input *TAGetTasksInput) (*TAGetTasksOutput, error) {
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

	return &TAGetTasksOutput{
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

// TAGetTaskInput 获取任务详情
type TAGetTaskInput struct {
	ID string `path:"id" description:"任务ID"`
}

// TAGetTaskOutput 获取任务详情
type TAGetTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// TAGetTask 获取任务详情
func (tc *TaskController) TAGetTask(ctx context.Context, input *TAGetTaskInput) (*TAGetTaskOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	task := tc.taskService.GetTaskByID(input.ID)
	if task == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	return &TAGetTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// TABulkSaveTaskInput 批量保存任务
type TABulkSaveTaskInput struct {
	Body []vo.TaskVO
}

// TABulkSaveTaskOutput 批量保存任务
type TABulkSaveTaskOutput struct {
	Body utils.HumaResponse[any]
}

// TABulkSaveTask 批量保存任务
func (tc *TaskController) TABulkSaveTask(ctx context.Context, input *TABulkSaveTaskInput) (*TABulkSaveTaskOutput, error) {
	reqs := input.Body
	for _, req := range reqs {
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
			WorkDir:       req.WorkDir,
			CleanConfig:   req.CleanConfig,
			Envs:          req.Envs,
			Languages:     req.Languages,
			AgentID:       req.AgentID,
			TriggerType:   req.TriggerType,
			RetryCount:    req.RetryCount,
			RetryInterval: req.RetryInterval,
			RandomRange:   req.RandomRange,
			PinType:       req.PinType,
			Enabled:       req.Enabled,
			SourceID:      "", // 不直接覆盖
		}

		var existingTask *models.Task
		// 优先按 ID 匹配
		if req.ID != "" {
			existingTask = tc.taskService.GetTaskByID(req.ID)
		}
		// 如果 ID 没找到，尝试按 Name 匹配
		if existingTask == nil {
			var t models.Task
			res := database.DB.Where("name = ?", req.Name).First(&t)
			if res.Error == nil {
				existingTask = &t
			}
		}

		var savedTask *models.Task
		if existingTask != nil {
			savedTask = tc.taskService.UpdateTask(existingTask.ID, &param)
		} else {
			savedTask = tc.taskService.CreateTask(&param)
			// 如果原始有 ID，强制覆盖更新 ID 保持强同步一致性
			if req.ID != "" && savedTask != nil {
				database.DB.Model(savedTask).Update("id", req.ID)
				savedTask.ID = req.ID
			}
		}

		// 如果是 Agent 任务，通知 Agent；否则添加到本地 cron
		if savedTask != nil {
			if savedTask.AgentID != nil && *savedTask.AgentID != "" {
				tc.agentWSManager.BroadcastTasks(*savedTask.AgentID)
			} else {
				tc.executorService.AddCronTask(savedTask)
			}
		}
	}

	return &TABulkSaveTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TAUpdateTaskInput 更新任务
type TAUpdateTaskInput struct {
	ID   string `path:"id" description:"任务ID"`
	Body vo.TaskUpdateReq
}

// TAUpdateTaskOutput 更新任务
type TAUpdateTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// TAUpdateTask 更新任务
func (tc *TaskController) TAUpdateTask(ctx context.Context, input *TAUpdateTaskInput) (*TAUpdateTaskOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	// 获取旧任务信息（用于判断 agent 变更）
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

	// 转换为绝对路径（Agent 任务保持原样）
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

			// 验证更新后的 SourceID 是否和别的任务冲突
			if oldTask != nil && sourceID != oldTask.SourceID {
				existingTask := tc.taskService.GetTaskBySourceID(sourceID)
				if existingTask != nil && existingTask.ID != oldTask.ID {
					return nil, utils.HumaBadRequest("当前任务已存在，请检查或更换仓库目录名称")
				}
			}

			// 计算新的物理路径
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

			// 如果路径发生了改变（或者是个全新计算的路径），并且新路径已存在，则报错拦截
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

	// 处理任务调度
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

	return &TAUpdateTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// TADeleteTaskInput 删除任务
type TADeleteTaskInput struct {
	ID          string `path:"id" description:"任务ID"`
	DeleteFiles bool   `query:"delete_files" description:"是否同时删除物理文件"`
}

// TADeleteTaskOutput 删除任务
type TADeleteTaskOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteTask 删除任务
func (tc *TaskController) TADeleteTask(ctx context.Context, input *TADeleteTaskInput) (*TADeleteTaskOutput, error) {
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

	return &TADeleteTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TABatchDeleteTasksInput 批量删除任务
type TABatchDeleteTasksInput struct {
	Body struct {
		IDs []string `json:"ids" description:"任务ID列表"`
	}
}

// TABatchDeleteTasksOutput 批量删除任务
type TABatchDeleteTasksOutput struct {
	Body utils.HumaResponse[struct {
		Count int64 `json:"count"`
	}]
}

// TABatchDeleteTasks 批量删除任务
func (tc *TaskController) TABatchDeleteTasks(ctx context.Context, input *TABatchDeleteTasksInput) (*TABatchDeleteTasksOutput, error) {
	req := input.Body
	// 收集涉及到的 AgentID
	agentIDs := make(map[string]struct{})
	for _, id := range req.IDs {
		task := tc.taskService.GetTaskByID(id)
		if task != nil {
			if task.AgentID != nil && *task.AgentID != "" {
				agentIDs[*task.AgentID] = struct{}{}
			}
		}

		tc.executorService.RemoveCronTask(id)
		tc.executorService.GetScheduler().StopTask(id)
	}

	// 执行批量删除
	count := tc.taskService.BatchDeleteTasks(req.IDs)

	// 通知受影响的 Agent
	for agentID := range agentIDs {
		tc.agentWSManager.BroadcastTasks(agentID)
	}

	return &TABatchDeleteTasksOutput{
		Body: utils.HumaResponse[struct {
			Count int64 `json:"count"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Count int64 `json:"count"`
			}{Count: count},
		},
	}, nil
}

// TABatchDeleteByQueryInput 根据查询条件批量删除任务
type TABatchDeleteByQueryInput struct {
	Name    string `query:"name" description:"任务名称关键词"`
	Tags    string `query:"tags" description:"标签关键词"`
	Type    string `query:"type" description:"任务类型"`
	AgentID string `query:"agent_id" description:"执行位置(节点ID)"`
}

// TABatchDeleteByQueryOutput 根据查询条件批量删除任务
type TABatchDeleteByQueryOutput struct {
	Body utils.HumaResponse[struct {
		Count int64 `json:"count"`
	}]
}

// TABatchDeleteByQuery 根据查询条件批量删除任务
func (tc *TaskController) TABatchDeleteByQuery(ctx context.Context, input *TABatchDeleteByQueryInput) (*TABatchDeleteByQueryOutput, error) {
	var agentID *string
	if input.AgentID != "" {
		id := input.AgentID
		agentID = &id
	}

	tasksList, _ := tc.taskService.GetTasksWithPagination(1, 999999, input.Name, agentID, input.Tags, input.Type, "", "")
	if len(tasksList) == 0 {
		return &TABatchDeleteByQueryOutput{
			Body: utils.HumaResponse[struct {
				Count int64 `json:"count"`
			}]{
				Code: 200,
				Msg:  "success",
				Data: struct {
					Count int64 `json:"count"`
				}{Count: 0},
			},
		}, nil
	}

	var ids []string
	agentIDs := make(map[string]struct{})
	for _, task := range tasksList {
		ids = append(ids, task.ID)
		if task.AgentID != nil && *task.AgentID != "" {
			agentIDs[*task.AgentID] = struct{}{}
		}
		tc.executorService.RemoveCronTask(task.ID)
		tc.executorService.GetScheduler().StopTask(task.ID)
	}

	count := tc.taskService.BatchDeleteTasks(ids)

	for aID := range agentIDs {
		tc.agentWSManager.BroadcastTasks(aID)
	}

	return &TABatchDeleteByQueryOutput{
		Body: utils.HumaResponse[struct {
			Count int64 `json:"count"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Count int64 `json:"count"`
			}{Count: count},
		},
	}, nil
}

// TAStopTaskInput 停止任务
type TAStopTaskInput struct {
	LogID string `path:"logID" description:"运行日志ID"`
}

// TAStopTaskOutput 停止任务
type TAStopTaskOutput struct {
	Body utils.HumaResponse[any]
}

// TAStopTask 停止任务
func (tc *TaskController) TAStopTask(ctx context.Context, input *TAStopTaskInput) (*TAStopTaskOutput, error) {
	logID := input.LogID
	if logID == "" {
		return nil, utils.HumaBadRequest("无效的日志ID")
	}

	err := tc.executorService.StopTaskExecution(logID)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &TAStopTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "停止请求已发送",
		},
	}, nil
}

// TAGetTaskTagsOutput 获取所有任务标签
type TAGetTaskTagsOutput struct {
	Body utils.HumaResponse[[]string]
}

// TAGetTaskTags 获取所有任务标签
func (tc *TaskController) TAGetTaskTags(ctx context.Context, input *struct{}) (*TAGetTaskTagsOutput, error) {
	tags, err := tc.taskService.GetAllTags()
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAGetTaskTagsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: tags,
		},
	}, nil
}

// RegisterAPITaskRoutes 注册 /api/v1 任务管理 Huma 路由
func (tc *TaskController) RegisterAPITaskRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tasks",
		OperationID: "apiCreateTask",
		Summary:     "创建任务",
		Description: "创建一个新的任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TACreateTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tasks",
		OperationID: "apiGetTasks",
		Summary:     "获取任务列表",
		Description: "分页获取任务列表，支持按名称、Agent ID、标签、类型筛选",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAGetTasks)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tasks/{id}",
		OperationID: "apiGetTask",
		Summary:     "获取任务详情",
		Description: "根据 ID 获取任务详情",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAGetTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tasks/bulk_save",
		OperationID: "apiBulkSaveTask",
		Summary:     "批量保存任务",
		Description: "批量导入任务配置，如果ID或同名存在则更新，不存在则创建",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TABulkSaveTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/tasks/{id}",
		OperationID: "apiUpdateTask",
		Summary:     "更新任务",
		Description: "根据 ID 更新任务信息",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAUpdateTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}",
		OperationID: "apiDeleteTask",
		Summary:     "删除任务",
		Description: "根据 ID 删除任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TADeleteTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tasks/batch-delete",
		OperationID: "apiBatchDeleteTasks",
		Summary:     "批量删除任务",
		Description: "根据 ID 列表批量删除任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TABatchDeleteTasks)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/tasks/batch-by-query",
		OperationID: "apiBatchDeleteByQuery",
		Summary:     "根据查询条件批量删除任务",
		Description: "根据查询条件批量删除匹配的所有任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TABatchDeleteByQuery)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/tasks/stop/{logID}",
		OperationID: "apiStopTask",
		Summary:     "停止任务",
		Description: "根据运行日志 ID 停止正在执行的任务",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAStopTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/tasks/tags",
		OperationID: "apiGetTaskTags",
		Summary:     "获取所有任务标签",
		Description: "获取系统中所有任务已使用的唯一标签列表",
		Tags:        []string{"任务管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TAGetTaskTags)
}

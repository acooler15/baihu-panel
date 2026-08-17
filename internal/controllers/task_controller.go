package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type TaskController struct {
	taskService     *tasks.TaskService
	executorService *tasks.ExecutorService
	agentWSManager  *services.AgentWSManager
}

func NewTaskController(taskService *tasks.TaskService, executorService *tasks.ExecutorService) *TaskController {
	return &TaskController{
		taskService:     taskService,
		executorService: executorService,
		agentWSManager:  services.GetAgentWSManager(),
	}
}

// resolveWorkDir 将相对路径转换为绝对路径
func resolveWorkDir(workDir string) string {
	if workDir == "" {
		// 空则使用默认 scripts 目录
		absPath, err := filepath.Abs(constant.ScriptsWorkDir)
		if err != nil {
			return constant.ScriptsWorkDir
		}
		return absPath
	}
	// 如果已经是绝对路径，直接返回
	if strings.HasPrefix(workDir, constant.ScriptsDirPlaceholder) {
		return workDir
	}
	if filepath.IsAbs(workDir) {
		return workDir
	}
	// 相对路径，基于 scripts 目录
	fullPath := filepath.Join(constant.ScriptsWorkDir, workDir)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fullPath
	}
	return absPath
}

// isValidDirName 校验目录名是否合法
func isValidDirName(dirName string) bool {
	if dirName == "." || strings.Contains(dirName, "/") || strings.Contains(dirName, "\\") || strings.Contains(dirName, "..") {
		return false
	}
	for _, ch := range dirName {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
			return false
		}
	}
	return true
}

// getRepoPhysicalPath 计算仓库任务的最终物理绝对路径
func getRepoPhysicalPath(targetPath, dirName, sourceURL, branch string) string {
	if dirName == "." {
		return "" // 如果不追加目录，此逻辑不负责判断其根目录（共享的 scripts 目录）
	}
	finalDirName := dirName
	if finalDirName == "" {
		finalDirName = utils.GetRepoIdentifier(sourceURL, branch)
	}
	if finalDirName == "" {
		return ""
	}

	basePath := targetPath
	if basePath == "" || basePath == constant.ScriptsDirPlaceholder {
		basePath = constant.ScriptsWorkDir
	} else if strings.HasPrefix(basePath, constant.ScriptsDirPlaceholder) {
		basePath = filepath.Join(constant.ScriptsWorkDir, strings.TrimPrefix(basePath, constant.ScriptsDirPlaceholder))
	} else if !filepath.IsAbs(basePath) {
		basePath = filepath.Join(constant.ScriptsWorkDir, basePath)
	}

	fullPath := filepath.Join(basePath, finalDirName)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return ""
	}
	return absPath
}

// deleteRepoPhysicalFiles 删除仓库关联的物理文件
func (tc *TaskController) deleteRepoPhysicalFiles(task *models.Task) {
	if task.Type != constant.TaskTypeRepo {
		return
	}

	logger.Infof("[Controller] 开始尝试物理删除任务关联文件: %s", task.Name)
	var repoCfg models.RepoConfig
	if err := json.Unmarshal([]byte(task.Config), &repoCfg); err != nil {
		logger.Errorf("[Controller] 解析任务配置失败: %v", err)
		return
	}

	targetPath := repoCfg.TargetPath
	if targetPath == "" {
		// 如果 TargetPath 为空，调用系统的计算函数获取默认目录名
		repoId := utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
		if repoId != "" {
			targetPath = repoId
			logger.Infof("[Controller] TargetPath 为空，使用计算出的标识符: %s", targetPath)
		}
	}

	if targetPath == "" || targetPath == constant.ScriptsDirPlaceholder {
		logger.Warnf("[Controller] 任务 %s 无法确定有效的物理删除路径，跳过", task.Name)
		return
	}

	// 确定绝对路径
	scriptsDir, _ := filepath.Abs(constant.ScriptsWorkDir)
	fullPath := targetPath
	if strings.HasPrefix(targetPath, constant.ScriptsDirPlaceholder) {
		fullPath = filepath.Join(scriptsDir, strings.TrimPrefix(targetPath, constant.ScriptsDirPlaceholder))
	} else if !filepath.IsAbs(targetPath) {
		fullPath = filepath.Join(scriptsDir, targetPath)
	}

	absTargetPath, _ := filepath.Abs(fullPath)
	logger.Infof("[Controller] 最终计算的绝对路径: %s, Scripts目录: %s", absTargetPath, scriptsDir)
	scriptsDir, _ = filepath.Abs(constant.ScriptsWorkDir)

	// 安全检查：使用 Rel 判断路径关系
	rel, err := filepath.Rel(scriptsDir, absTargetPath)
	if err != nil {
		logger.Errorf("[Controller] 计算相对路径失败: %v", err)
		return
	}

	// 必须是在 scripts 目录下（不以 .. 开头）且不能是 scripts 目录本身 (.)
	if rel != "." && !strings.HasPrefix(rel, "..") {
		err := os.RemoveAll(absTargetPath)
		if err != nil {
			logger.Errorf("[Controller] 物理删除文件夹失败: %s, 路径: %s, 错误: %v", task.Name, absTargetPath, err)
		} else {
			logger.Infof("[Controller] 已成功物理删除文件夹: %s, 路径: %s", task.Name, absTargetPath)
		}
	} else {
		logger.Warnf("[Controller] 拒绝物理删除安全目录之外的路径: %s", absTargetPath)
	}
}

// ===========================================================================
// Gin 原生 handler（由 api_routes.go 保留引用）
// ===========================================================================

// ===========================================================================
// 任务管理业务方法（Huma）
// ===========================================================================

// CreateTaskInput 创建任务
type CreateTaskInput struct {
	Body vo.TaskCreateReq
}

// CreateTaskOutput 创建任务
type CreateTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// CreateTask 创建任务
func (tc *TaskController) CreateTask(ctx context.Context, input *CreateTaskInput) (*CreateTaskOutput, error) {
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

	return &CreateTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// GetTasksInput 获取任务列表
type GetTasksInput struct {
	Name     string `query:"name" description:"任务名称"`
	AgentID  string `query:"agent_id" description:"Agent ID"`
	Tags     string `query:"tags" description:"标签"`
	Type     string `query:"type" description:"任务类型"`
	SortBy   string `query:"sort_by" description:"排序字段"`
	Order    string `query:"order" description:"排序方向"`
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
}

// GetTasksOutput 获取任务列表
type GetTasksOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.TaskVO]]
}

// GetTasks 获取任务列表
func (tc *TaskController) GetTasks(ctx context.Context, input *GetTasksInput) (*GetTasksOutput, error) {
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

	return &GetTasksOutput{
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

// GetTaskInput 获取任务详情
type GetTaskInput struct {
	ID string `path:"id" description:"任务ID"`
}

// GetTaskOutput 获取任务详情
type GetTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// GetTask 获取任务详情
func (tc *TaskController) GetTask(ctx context.Context, input *GetTaskInput) (*GetTaskOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	task := tc.taskService.GetTaskByID(input.ID)
	if task == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	return &GetTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// BulkSaveTaskInput 批量保存任务
type BulkSaveTaskInput struct {
	Body []vo.TaskVO
}

// BulkSaveTaskOutput 批量保存任务
type BulkSaveTaskOutput struct {
	Body utils.HumaResponse[any]
}

// BulkSaveTask 批量保存任务
func (tc *TaskController) BulkSaveTask(ctx context.Context, input *BulkSaveTaskInput) (*BulkSaveTaskOutput, error) {
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

	return &BulkSaveTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// UpdateTaskInput 更新任务
type UpdateTaskInput struct {
	ID   string `path:"id" description:"任务ID"`
	Body vo.TaskUpdateReq
}

// UpdateTaskOutput 更新任务
type UpdateTaskOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// UpdateTask 更新任务
func (tc *TaskController) UpdateTask(ctx context.Context, input *UpdateTaskInput) (*UpdateTaskOutput, error) {
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

	return &UpdateTaskOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(task),
		},
	}, nil
}

// DeleteTaskInput 删除任务
type DeleteTaskInput struct {
	ID          string `path:"id" description:"任务ID"`
	DeleteFiles bool   `query:"delete_files" description:"是否同时删除物理文件"`
}

// DeleteTaskOutput 删除任务
type DeleteTaskOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteTask 删除任务
func (tc *TaskController) DeleteTask(ctx context.Context, input *DeleteTaskInput) (*DeleteTaskOutput, error) {
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

	return &DeleteTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// BatchDeleteTasksInput 批量删除任务
type BatchDeleteTasksInput struct {
	Body struct {
		IDs []string `json:"ids" description:"任务ID列表"`
	}
}

// BatchDeleteTasksOutput 批量删除任务
type BatchDeleteTasksOutput struct {
	Body utils.HumaResponse[struct {
		Count int64 `json:"count"`
	}]
}

// BatchDeleteTasks 批量删除任务
func (tc *TaskController) BatchDeleteTasks(ctx context.Context, input *BatchDeleteTasksInput) (*BatchDeleteTasksOutput, error) {
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

	return &BatchDeleteTasksOutput{
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

// BatchDeleteByQueryInput 根据查询条件批量删除任务
type BatchDeleteByQueryInput struct {
	Name    string `query:"name" description:"任务名称关键词"`
	Tags    string `query:"tags" description:"标签关键词"`
	Type    string `query:"type" description:"任务类型"`
	AgentID string `query:"agent_id" description:"执行位置(节点ID)"`
}

// BatchDeleteByQueryOutput 根据查询条件批量删除任务
type BatchDeleteByQueryOutput struct {
	Body utils.HumaResponse[struct {
		Count int64 `json:"count"`
	}]
}

// BatchDeleteByQuery 根据查询条件批量删除任务
func (tc *TaskController) BatchDeleteByQuery(ctx context.Context, input *BatchDeleteByQueryInput) (*BatchDeleteByQueryOutput, error) {
	var agentID *string
	if input.AgentID != "" {
		id := input.AgentID
		agentID = &id
	}

	tasksList, _ := tc.taskService.GetTasksWithPagination(1, 999999, input.Name, agentID, input.Tags, input.Type, "", "")
	if len(tasksList) == 0 {
		return &BatchDeleteByQueryOutput{
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

	return &BatchDeleteByQueryOutput{
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

// StopTaskInput 停止任务
type StopTaskInput struct {
	LogID string `path:"logID" description:"运行日志ID"`
}

// StopTaskOutput 停止任务
type StopTaskOutput struct {
	Body utils.HumaResponse[any]
}

// StopTask 停止任务
func (tc *TaskController) StopTask(ctx context.Context, input *StopTaskInput) (*StopTaskOutput, error) {
	logID := input.LogID
	if logID == "" {
		return nil, utils.HumaBadRequest("无效的日志ID")
	}

	err := tc.executorService.StopTaskExecution(logID)
	if err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &StopTaskOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "停止请求已发送",
		},
	}, nil
}

// GetTaskTagsOutput 获取所有任务标签
type GetTaskTagsOutput struct {
	Body utils.HumaResponse[[]string]
}

// GetTaskTags 获取所有任务标签
func (tc *TaskController) GetTaskTags(ctx context.Context, input *struct{}) (*GetTaskTagsOutput, error) {
	tags, err := tc.taskService.GetAllTags()
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &GetTaskTagsOutput{
		Body: utils.HumaResponse[[]string]{
			Code: 200,
			Msg:  "success",
			Data: tags,
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// registerTaskRoutes 注册任务管理共用路由（7 条）。
// security 直接作为 OpenAPI 文档的 Security 声明。
func (tc *TaskController) registerTaskRoutes(api huma.API, security []map[string][]string) {
	tag := []string{"任务管理"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tasks", OperationID: "CreateTask", Summary: "创建任务", Description: "创建一个新的任务", Tags: tag, Security: security}, tc.CreateTask)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/tasks", OperationID: "GetTasks", Summary: "获取任务列表", Description: "分页获取任务列表，支持按名称、Agent ID、标签、类型筛选", Tags: tag, Security: security}, tc.GetTasks)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/tasks/{id}", OperationID: "GetTask", Summary: "获取任务详情", Description: "根据 ID 获取任务详情", Tags: tag, Security: security}, tc.GetTask)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/tasks/{id}", OperationID: "UpdateTask", Summary: "更新任务", Description: "根据 ID 更新任务信息", Tags: tag, Security: security}, tc.UpdateTask)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/tasks/{id}", OperationID: "DeleteTask", Summary: "删除任务", Description: "根据 ID 删除任务", Tags: tag, Security: security}, tc.DeleteTask)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tasks/stop/{logID}", OperationID: "StopTask", Summary: "停止任务", Description: "根据运行日志 ID 停止正在执行的任务", Tags: tag, Security: security}, tc.StopTask)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/tasks/tags", OperationID: "GetTaskTags", Summary: "获取所有任务标签", Description: "获取系统中所有任务已使用的唯一标签列表", Tags: tag, Security: security}, tc.GetTaskTags)
}

// RegisterAPITaskRoutes 注册 /api/v1 任务管理 Huma 路由（CookieAuth）
// 含独有接口：BulkSaveTask、BatchDeleteTasks、BatchDeleteByQuery
func (tc *TaskController) RegisterAPITaskRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"任务管理"}

	// 独有接口
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tasks/bulk_save", OperationID: "BulkSaveTask", Summary: "批量保存任务", Description: "批量导入任务配置，如果ID或同名存在则更新，不存在则创建", Tags: tag, Security: security}, tc.BulkSaveTask)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tasks/batch-delete", OperationID: "BatchDeleteTasks", Summary: "批量删除任务", Description: "根据 ID 列表批量删除任务", Tags: tag, Security: security}, tc.BatchDeleteTasks)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/tasks/batch-by-query", OperationID: "BatchDeleteByQuery", Summary: "根据查询条件批量删除任务", Description: "根据查询条件批量删除匹配的所有任务", Tags: tag, Security: security}, tc.BatchDeleteByQuery)

	// 共用接口
	tc.registerTaskRoutes(api, security)

	// 内部接口（LocalhostOnly，selector 中按路径放行）
	tc.registerInternalTaskRoutes(api)
}

// registerInternalTaskRoutes 注册任务管理的内部接口（仅本地回环调用）
func (tc *TaskController) registerInternalTaskRoutes(api huma.API) {
	tag := []string{"任务管理"}
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/internal/tasks/sync-repo-status", OperationID: "InternalSyncRepoTasks", Summary: "增量同步仓库任务状态", Description: "供本地 reposync 进程调用，增量同步仓库任务状态。", Tags: tag}, tc.SyncRepoTasksHuma)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/internal/tasks/toggle/{id}", OperationID: "InternalToggleTask", Summary: "切换任务启用状态", Description: "内部接口：切换任务的启用/禁用状态。", Tags: tag}, tc.ToggleTaskHuma)
}

// RegisterOpenAPITaskRoutes 注册 OpenAPI 任务管理 Huma 路由（BearerAuth，子集）
func (tc *TaskController) RegisterOpenAPITaskRoutes(api huma.API) {
	tc.registerTaskRoutes(api, []map[string][]string{{"BearerAuth": {}}})
}

// ===========================================================================
// 任务管理内部接口（Huma，迁移自 Gin 原生 SyncRepoTasks/ToggleTask）
// ===========================================================================

// SyncRepoTasksHumaInput 增量同步仓库任务状态请求
type SyncRepoTasksHumaInput struct {
	Body struct {
		RepoID      string   `json:"repo_id"`
		UpsertedIDs []string `json:"upserted_ids"`
		DeletedIDs  []string `json:"deleted_ids"`
	}
}

// SyncRepoTasksHumaOutput 增量同步结果
type SyncRepoTasksHumaOutput struct {
	Body utils.HumaResponse[any]
}

// SyncRepoTasksHuma 增量同步仓库任务状态（内部接口）
func (tc *TaskController) SyncRepoTasksHuma(ctx context.Context, input *SyncRepoTasksHumaInput) (*SyncRepoTasksHumaOutput, error) {
	req := input.Body
	tc.executorService.SyncRepoTasks(req.UpsertedIDs, req.DeletedIDs)
	return &SyncRepoTasksHumaOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "增量同步成功",
		},
	}, nil
}

// ToggleTaskHumaInput 切换任务启用状态请求
type ToggleTaskHumaInput struct {
	ID   string `path:"id" description:"任务 ID"`
	Body struct {
		Enabled bool `json:"enabled"`
	}
}

// ToggleTaskHumaOutput 切换结果
type ToggleTaskHumaOutput struct {
	Body utils.HumaResponse[*vo.TaskVO]
}

// ToggleTaskHuma 切换任务启用/禁用状态（内部接口）
func (tc *TaskController) ToggleTaskHuma(ctx context.Context, input *ToggleTaskHumaInput) (*ToggleTaskHumaOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	// 获取旧 AgentID
	var oldAgentID *string
	oldAgentID = task.AgentID

	// 构造更新参数，仅修改 Enabled
	param := tasks.TaskParam{
		Name:          task.Name,
		Remark:        task.Remark,
		Command:       string(task.Command),
		PreCommand:    string(task.PreCommand),
		PostCommand:   string(task.PostCommand),
		Tags:          task.Tags,
		Type:          task.Type,
		Config:        string(task.Config),
		Schedule:      task.Schedule,
		Timeout:       task.Timeout,
		WorkDir:       task.WorkDir,
		CleanConfig:   task.CleanConfig,
		Envs:          string(task.Envs),
		Languages:     task.Languages,
		AgentID:       task.AgentID,
		TriggerType:   task.TriggerType,
		RetryCount:    task.RetryCount,
		RetryInterval: task.RetryInterval,
		RandomRange:   task.RandomRange,
		SourceID:      task.SourceID,
		PinType:       task.PinType,
		Enabled:       input.Body.Enabled,
	}

	updatedTask := tc.taskService.UpdateTask(id, &param)
	if updatedTask == nil {
		return nil, utils.HumaNotFound("任务不存在")
	}

	// 处理调度器更新
	if updatedTask.AgentID != nil && *updatedTask.AgentID != "" {
		tc.executorService.RemoveCronTask(updatedTask.ID)
		tc.agentWSManager.BroadcastTasks(*updatedTask.AgentID)
	} else {
		if input.Body.Enabled {
			tc.executorService.AddCronTask(updatedTask)
		} else {
			tc.executorService.RemoveCronTask(updatedTask.ID)
		}
		if oldAgentID != nil && *oldAgentID != "" {
			tc.agentWSManager.BroadcastTasks(*oldAgentID)
		}
	}

	return &ToggleTaskHumaOutput{
		Body: utils.HumaResponse[*vo.TaskVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToTaskVO(updatedTask),
		},
	}, nil
}

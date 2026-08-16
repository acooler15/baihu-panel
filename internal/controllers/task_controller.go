package controllers

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
	"net/http"
	"os"
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

// SyncRepoTasks 增量同步仓库任务状态（供本地 reposync 进程调用）
func (tc *TaskController) SyncRepoTasks(c *gin.Context) {
	var req struct {
		RepoID      string   `json:"repo_id"`
		UpsertedIDs []string `json:"upserted_ids"`
		DeletedIDs  []string `json:"deleted_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: err.Error()})
		return
	}

	tc.executorService.SyncRepoTasks(req.UpsertedIDs, req.DeletedIDs)
	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "增量同步成功"})
}

// ToggleTask 切换任务启用/禁用状态
func (tc *TaskController) ToggleTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "无效的任务ID"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: err.Error()})
		return
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "任务不存在"})
		return
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
		Enabled:       req.Enabled,
	}

	updatedTask := tc.taskService.UpdateTask(id, &param)
	if updatedTask == nil {
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "任务不存在"})
		return
	}

	// 处理调度器更新
	if updatedTask.AgentID != nil && *updatedTask.AgentID != "" {
		tc.executorService.RemoveCronTask(updatedTask.ID)
		tc.agentWSManager.BroadcastTasks(*updatedTask.AgentID)
	} else {
		if req.Enabled {
			tc.executorService.AddCronTask(updatedTask)
		} else {
			tc.executorService.RemoveCronTask(updatedTask.ID)
		}
		if oldAgentID != nil && *oldAgentID != "" {
			tc.agentWSManager.BroadcastTasks(*oldAgentID)
		}
	}

	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "success", Data: vo.ToTaskVO(updatedTask)})
}

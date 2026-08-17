package vo

import (
	"github.com/engigu/baihu-panel/internal/executor"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"
)

// TaskCreateReq 任务创建请求
type TaskCreateReq struct {
	Name          string               `json:"name" binding:"required" example:"测试任务" description:"任务名称"`
	Remark        string               `json:"remark" example:"备注信息" description:"备注信息"`
	Command       string               `json:"command" example:"echo 'Hello World'" description:"执行命令"`
	PreCommand    string               `json:"pre_command" example:"echo 'pre'" description:"执行前命令"`
	PostCommand   string               `json:"post_command" example:"echo 'post'" description:"执行后命令"`
	Tags          string               `json:"tags" example:"test,dev" description:"标签"`
	Type          string               `json:"type" example:"repo" description:"任务类型，common 或 repo"`
	Config        string               `json:"config" example:"{\"source_url\":\"https://github.com/abc/repo\",\"branch\":\"main\"}" description:"仓库任务配置"`
	Schedule      string               `json:"schedule" example:"0 0 * * *" description:"cron 表达式"`
	Timeout       int                  `json:"timeout" example:"3600" description:"超时时间(秒)"`
	WorkDir       string               `json:"work_dir" example:"/tmp" description:"工作目录"`
	CleanConfig   string               `json:"clean_config" example:"true" description:"清理配置"`
	Envs          string               `json:"envs" example:"{\"ENV_VAR\":\"value\"}" description:"环境变量"`
	Languages     models.TaskLanguages `json:"languages" description:"语言环境"`
	AgentID       *string              `json:"agent_id" example:"agent-1" description:"执行节点ID"`
	TriggerType   string               `json:"trigger_type" example:"cron" description:"触发方式"`
	RetryCount    int                  `json:"retry_count" example:"3" description:"重试次数"`
	RetryInterval int                  `json:"retry_interval" example:"60" description:"重试间隔(秒)"`
	RandomRange   int                  `json:"random_range" example:"10" description:"随机范围"`
	PinType       string               `json:"pin_type" example:"time" description:"固定类型"`
}

// TaskUpdateReq 任务更新请求
type TaskUpdateReq struct {
	Name          string               `json:"name" example:"测试任务" description:"任务名称"`
	Remark        string               `json:"remark" example:"备注信息" description:"备注信息"`
	Command       string               `json:"command" example:"echo 'Hello World'" description:"执行命令"`
	PreCommand    string               `json:"pre_command" example:"echo 'pre'" description:"执行前命令"`
	PostCommand   string               `json:"post_command" example:"echo 'post'" description:"执行后命令"`
	Tags          string               `json:"tags" example:"test,dev" description:"标签"`
	Type          string               `json:"type" example:"repo" description:"任务类型，common 或 repo"`
	Config        string               `json:"config" example:"{\"source_url\":\"https://github.com/abc/repo\",\"branch\":\"main\"}" description:"仓库任务配置"`
	Schedule      string               `json:"schedule" example:"0 0 * * *" description:"cron 表达式"`
	Timeout       int                  `json:"timeout" example:"3600" description:"超时时间(秒)"`
	WorkDir       string               `json:"work_dir" example:"/tmp" description:"工作目录"`
	CleanConfig   string               `json:"clean_config" example:"true" description:"清理配置"`
	Envs          string               `json:"envs" example:"{\"ENV_VAR\":\"value\"}" description:"环境变量"`
	Enabled       bool                 `json:"enabled" example:"true" description:"是否启用"`
	Languages     models.TaskLanguages `json:"languages" description:"语言环境"`
	AgentID       *string              `json:"agent_id" example:"agent-1" description:"执行节点ID"`
	TriggerType   string               `json:"trigger_type" example:"cron" description:"触发方式"`
	RetryCount    int                  `json:"retry_count" example:"3" description:"重试次数"`
	RetryInterval int                  `json:"retry_interval" example:"60" description:"重试间隔(秒)"`
	RandomRange   int                  `json:"random_range" example:"10" description:"随机范围"`
	PinType       string               `json:"pin_type" example:"time" description:"固定类型"`
}

// TaskVO 任务视图对象
type TaskVO struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Remark        string               `json:"remark"`
	Command       string               `json:"command"`
	PreCommand    string               `json:"pre_command"`
	PostCommand   string               `json:"post_command"`
	Tags          string               `json:"tags"`
	Type          string               `json:"type"`
	TriggerType   string               `json:"trigger_type"`
	Config        string               `json:"config"`
	Schedule      string               `json:"schedule"`
	Timeout       int                  `json:"timeout"`
	WorkDir       string               `json:"work_dir"`
	CleanConfig   string               `json:"clean_config"`
	Envs          string               `json:"envs"`
	Languages     models.TaskLanguages `json:"languages"`
	AgentID       *string              `json:"agent_id"`
	RepoTaskID    string               `json:"repo_task_id"`
	Enabled       bool                 `json:"enabled"`
	RetryCount    int                  `json:"retry_count"`
	RetryInterval int                  `json:"retry_interval"`
	RandomRange   int                  `json:"random_range"`
	PinType       string               `json:"pin_type"`
	LastRun       *models.LocalTime    `json:"last_run"`
	NextRun       *models.LocalTime    `json:"next_run"`
	CreatedAt     models.LocalTime     `json:"created_at"`
	UpdatedAt     models.LocalTime     `json:"updated_at"`
	RunningStatus string               `json:"running_status"`
}

// ToTaskVO 将 Task 模型转换为 TaskVO
func ToTaskVO(task *models.Task) *TaskVO {
	if task == nil {
		return nil
	}
	return &TaskVO{
		ID:            task.ID,
		Name:          task.Name,
		Remark:        task.Remark,
		Command:       string(task.Command),
		PreCommand:    string(task.PreCommand),
		PostCommand:   string(task.PostCommand),
		Tags:          task.Tags,
		Type:          task.Type,
		TriggerType:   task.TriggerType,
		Config:        string(task.Config),
		Schedule:      task.Schedule,
		Timeout:       task.Timeout,
		WorkDir:       task.WorkDir,
		CleanConfig:   task.CleanConfig,
		Envs:          string(task.Envs),
		Languages:     task.Languages,
		AgentID:       task.AgentID,
		RepoTaskID:    task.RepoTaskID,
		Enabled:       utils.DerefBool(task.Enabled, true),
		RetryCount:    task.RetryCount,
		RetryInterval: task.RetryInterval,
		RandomRange:   task.RandomRange,
		PinType:       task.PinType,
		LastRun:       task.LastRun,
		NextRun:       task.NextRun,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
		RunningStatus: func() string {
			if task.IsRunning() {
				return "running"
			}
			return "idle"
		}(),
	}
}

// ToTaskVOList 将 Task 模型列表转换为 TaskVO 列表
func ToTaskVOList(tasks []*models.Task) []*TaskVO {
	if tasks == nil {
		return nil
	}
	vos := make([]*TaskVO, len(tasks))
	for i, t := range tasks {
		vos[i] = ToTaskVO(t)
	}
	return vos
}

// ToTaskVOListFromModels 将 Task 模型列表转换为 TaskVO 列表
func ToTaskVOListFromModels(tasks []models.Task) []*TaskVO {
	vos := make([]*TaskVO, len(tasks))
	for i := range tasks {
		vos[i] = ToTaskVO(&tasks[i])
	}
	return vos
}

// TaskLogVO 任务历史视图对象
type TaskLogVO struct {
	ID        string            `json:"id" description:"日志ID"`
	TaskID    string            `json:"task_id" description:"任务ID"`
	TaskName  string            `json:"task_name" description:"任务名称"`
	TaskType  string            `json:"task_type" description:"任务类型"`
	AgentID   *string           `json:"agent_id" description:"执行节点ID"`
	Command   string            `json:"command" description:"执行命令"`
	Error     string            `json:"error" description:"错误信息"`
	Status    string            `json:"status" description:"执行状态"`
	Duration  int64             `json:"duration" description:"执行耗时(毫秒)"`
	ExitCode  int               `json:"exit_code" description:"退出码"`
	StartTime *models.LocalTime `json:"start_time" description:"开始时间"`
	EndTime   *models.LocalTime `json:"end_time" description:"结束时间"`
	CreatedAt models.LocalTime  `json:"created_at" description:"创建时间"`
	Output    string            `json:"output,omitempty" description:"执行输出"`
}

// ToTaskLogVO 将 TaskLog 模型转换为 TaskLogVO
// Note: This function assumes the Task field within models.TaskLog is preloaded
// or that taskName and taskType are provided from an external source.
func ToTaskLogVO(log *models.TaskLog) *TaskLogVO {
	if log == nil {
		return nil
	}
	return &TaskLogVO{
		ID:        log.ID,
		TaskID:    log.TaskID,
		AgentID:   log.AgentID,
		Command:   string(log.Command),
		Error:     string(log.Error),
		Status:    log.Status,
		Duration:  log.Duration,
		ExitCode:  log.ExitCode,
		StartTime: log.StartTime,
		EndTime:   log.EndTime,
		CreatedAt: log.CreatedAt,
		Output:    string(log.Output),
	}
}

// ToTaskLogVOList 将 TaskLog 模型列表转换为 TaskLogVO 列表
func ToTaskLogVOList(logs []*models.TaskLog) []*TaskLogVO {
	if logs == nil {
		return nil
	}
	vos := make([]*TaskLogVO, len(logs))
	for i, l := range logs {
		vos[i] = ToTaskLogVO(l)
	}
	return vos
}

// ToTaskLogVOListFromModels 将 TaskLog 模型列表转换为 TaskLogVO 列表
func ToTaskLogVOListFromModels(logs []models.TaskLog) []*TaskLogVO {
	vos := make([]*TaskLogVO, len(logs))
	for i := range logs {
		vos[i] = ToTaskLogVO(&logs[i])
	}
	return vos
}

// ExecuteTaskReq 执行任务请求
type ExecuteTaskReq struct {
	Envs map[string]string `json:"envs" description:"执行时注入的环境变量"`
}

// ExecutionResultVO 任务执行结果视图对象
type ExecutionResultVO struct {
	TaskID    string `json:"task_id" description:"任务ID"`
	LogID     string `json:"log_id,omitempty" description:"日志ID"`
	Success   bool   `json:"success" description:"是否成功"`
	Status    string `json:"status" description:"执行状态"`
	Output    string `json:"output,omitempty" description:"执行输出"`
	Error     string `json:"error,omitempty" description:"错误信息"`
	Duration  int64  `json:"duration,omitempty" description:"执行耗时(毫秒)"`
	ExitCode  int    `json:"exit_code,omitempty" description:"退出码"`
	StartTime string `json:"start_time,omitempty" description:"开始时间"`
	EndTime   string `json:"end_time,omitempty" description:"结束时间"`
}

// ToExecutionResultVO 将 ExecutionResult 转换为 ExecutionResultVO
func ToExecutionResultVO(res *executor.ExecutionResult) *ExecutionResultVO {
	if res == nil {
		return nil
	}
	vo := &ExecutionResultVO{
		TaskID:   res.TaskID,
		LogID:    res.LogID,
		Success:  res.Success,
		Status:   res.Status,
		Output:   res.Output,
		Error:    res.Error,
		Duration: res.Duration,
		ExitCode: res.ExitCode,
	}
	if !res.StartTime.IsZero() {
		vo.StartTime = res.StartTime.Format("2006-01-02 15:04:05")
	}
	if !res.EndTime.IsZero() {
		vo.EndTime = res.EndTime.Format("2006-01-02 15:04:05")
	}
	return vo
}

// ToExecutionResultVOList 将 ExecutionResult 列表转换为 ExecutionResultVO 列表
func ToExecutionResultVOList(results []executor.ExecutionResult) []*ExecutionResultVO {
	if results == nil {
		return nil
	}
	vos := make([]*ExecutionResultVO, len(results))
	for i := range results {
		vos[i] = ToExecutionResultVO(&results[i])
	}
	return vos
}

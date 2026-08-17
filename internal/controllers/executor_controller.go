package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type ExecutorController struct {
	executorService *tasks.ExecutorService
}

func NewExecutorController(executorService *tasks.ExecutorService) *ExecutorController {
	return &ExecutorController{executorService: executorService}
}

// ===========================================================================
// Gin 原生 handler（由 api_routes.go 保留引用）
// ===========================================================================

// ===========================================================================
// 任务执行业务方法（Huma）
// ===========================================================================

// RunTaskInput 运行任务
type RunTaskInput struct {
	ID   string            `path:"id" description:"任务ID"`
	Body vo.ExecuteTaskReq // body 可选，无必填字段
}

// RunTaskOutput 运行任务
type RunTaskOutput struct {
	Body utils.HumaResponse[*vo.ExecutionResultVO]
}

// RunTask 运行任务
func (ec *ExecutorController) RunTask(ctx context.Context, input *RunTaskInput) (*RunTaskOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	var extraEnvs []string
	if input.Body.Envs != nil {
		for k, v := range input.Body.Envs {
			extraEnvs = append(extraEnvs, k+"="+v)
		}
	}

	result := ec.executorService.ExecuteTask(input.ID, extraEnvs)

	return &RunTaskOutput{
		Body: utils.HumaResponse[*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVO(result),
		},
	}, nil
}

// ExecuteCommandInput 执行命令
type ExecuteCommandInput struct {
	Body struct {
		Command string `json:"command" description:"要执行的命令"`
	}
}

// ExecuteCommandOutput 执行命令
type ExecuteCommandOutput struct {
	Body utils.HumaResponse[*vo.ExecutionResultVO]
}

// ExecuteCommand 执行命令
func (ec *ExecutorController) ExecuteCommand(ctx context.Context, input *ExecuteCommandInput) (*ExecuteCommandOutput, error) {
	if input.Body.Command == "" {
		return nil, utils.HumaBadRequest("命令不能为空")
	}

	result := ec.executorService.ExecuteCommand(input.Body.Command)

	return &ExecuteCommandOutput{
		Body: utils.HumaResponse[*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVO(result),
		},
	}, nil
}

// GetLastResultsInput 获取最新执行结果
type GetLastResultsInput struct {
	Count int `query:"count" default:"10" description:"数量"`
}

// GetLastResultsOutput 获取最新执行结果
type GetLastResultsOutput struct {
	Body utils.HumaResponse[[]*vo.ExecutionResultVO]
}

// GetLastResults 获取最新执行结果
func (ec *ExecutorController) GetLastResults(ctx context.Context, input *GetLastResultsInput) (*GetLastResultsOutput, error) {
	count := input.Count
	if count <= 0 {
		count = 10
	}

	results := ec.executorService.GetLastResults(count)

	return &GetLastResultsOutput{
		Body: utils.HumaResponse[[]*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVOList(results),
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// registerExecutorRoutes 注册任务执行共用路由（2 条）。
// security 直接作为 OpenAPI 文档的 Security 声明。
func (ec *ExecutorController) registerExecutorRoutes(api huma.API, security []map[string][]string) {
	tag := []string{"任务执行"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/execute/task/{id}", OperationID: "RunTask", Summary: "运行任务", Description: "根据任务 ID 立即运行任务", Tags: tag, Security: security}, ec.RunTask)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/execute/results", OperationID: "GetLastResults", Summary: "获取最新执行结果", Description: "获取最近的任务执行结果", Tags: tag, Security: security}, ec.GetLastResults)
}

// RegisterAPIExecutorRoutes 注册 /api/v1 任务执行 Huma 路由（CookieAuth）
// 含独有接口：ExecuteCommand
func (ec *ExecutorController) RegisterAPIExecutorRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"任务执行"}

	// 独有接口
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/execute/command", OperationID: "ExecuteCommand", Summary: "执行命令", Description: "在服务器上执行一条命令", Tags: tag, Security: security}, ec.ExecuteCommand)

	// 共用接口
	ec.registerExecutorRoutes(api, security)

	// 内部接口（LocalhostOnly，selector 中按路径放行）
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/internal/tasks/execute/{id}", OperationID: "InternalExecuteTask", Summary: "运行任务（内部）", Description: "内部接口：根据任务 ID 立即运行任务，可选传入额外环境变量。", Tags: tag}, ec.ExecuteTaskHuma)
}

// RegisterOpenAPIExecutorRoutes 注册 OpenAPI 任务执行 Huma 路由（BearerAuth，子集）
func (ec *ExecutorController) RegisterOpenAPIExecutorRoutes(api huma.API) {
	ec.registerExecutorRoutes(api, []map[string][]string{{"BearerAuth": {}}})
}

// ===========================================================================
// 任务执行内部接口（Huma，迁移自 Gin 原生 ExecuteTask）
// ===========================================================================

// ExecuteTaskHumaInput 运行任务请求（内部）
type ExecuteTaskHumaInput struct {
	ID   string `path:"id" description:"任务 ID"`
	Body struct {
		Envs map[string]string `json:"envs" description:"额外环境变量"`
	}
}

// ExecuteTaskHumaOutput 运行任务结果
type ExecuteTaskHumaOutput struct {
	Body utils.HumaResponse[*vo.ExecutionResultVO]
}

// ExecuteTaskHuma 运行任务（内部接口）
func (ec *ExecutorController) ExecuteTaskHuma(ctx context.Context, input *ExecuteTaskHumaInput) (*ExecuteTaskHumaOutput, error) {
	id := input.ID
	if id == "" {
		return nil, utils.HumaBadRequest("无效的任务ID")
	}

	var extraEnvs []string
	if input.Body.Envs != nil {
		for k, v := range input.Body.Envs {
			extraEnvs = append(extraEnvs, k+"="+v)
		}
	}

	result := ec.executorService.ExecuteTask(id, extraEnvs)
	return &ExecuteTaskHumaOutput{
		Body: utils.HumaResponse[*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVO(result),
		},
	}, nil
}

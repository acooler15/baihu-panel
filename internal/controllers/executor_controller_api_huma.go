package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 任务执行
// ===========================================================================

// TAExecuteTaskInput 运行任务
type TAExecuteTaskInput struct {
	ID   string `path:"id" description:"任务ID"`
	Body vo.ExecuteTaskReq
}

// TAExecuteTaskOutput 运行任务
type TAExecuteTaskOutput struct {
	Body utils.HumaResponse[*vo.ExecutionResultVO]
}

// TAExecuteTask 运行任务
func (ec *ExecutorController) TAExecuteTask(ctx context.Context, input *TAExecuteTaskInput) (*TAExecuteTaskOutput, error) {
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

	return &TAExecuteTaskOutput{
		Body: utils.HumaResponse[*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVO(result),
		},
	}, nil
}

// TAExecuteCommandInput 执行命令
type TAExecuteCommandInput struct {
	Body struct {
		Command string `json:"command" description:"要执行的命令"`
	}
}

// TAExecuteCommandOutput 执行命令
type TAExecuteCommandOutput struct {
	Body utils.HumaResponse[*vo.ExecutionResultVO]
}

// TAExecuteCommand 执行命令
func (ec *ExecutorController) TAExecuteCommand(ctx context.Context, input *TAExecuteCommandInput) (*TAExecuteCommandOutput, error) {
	if input.Body.Command == "" {
		return nil, utils.HumaBadRequest("命令不能为空")
	}

	result := ec.executorService.ExecuteCommand(input.Body.Command)

	return &TAExecuteCommandOutput{
		Body: utils.HumaResponse[*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVO(result),
		},
	}, nil
}

// TAGetLastResultsInput 获取最新执行结果
type TAGetLastResultsInput struct {
	Count int `query:"count" default:"10" description:"数量"`
}

// TAGetLastResultsOutput 获取最新执行结果
type TAGetLastResultsOutput struct {
	Body utils.HumaResponse[[]*vo.ExecutionResultVO]
}

// TAGetLastResults 获取最新执行结果
func (ec *ExecutorController) TAGetLastResults(ctx context.Context, input *TAGetLastResultsInput) (*TAGetLastResultsOutput, error) {
	count := input.Count
	if count <= 0 {
		count = 10
	}

	results := ec.executorService.GetLastResults(count)

	return &TAGetLastResultsOutput{
		Body: utils.HumaResponse[[]*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVOList(results),
		},
	}, nil
}

// RegisterAPIExecutorRoutes 注册 /api/v1 任务执行 Huma 路由
func (ec *ExecutorController) RegisterAPIExecutorRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/execute/task/{id}",
		OperationID: "apiExecuteTask",
		Summary:     "运行任务",
		Description: "根据任务 ID 立即运行任务",
		Tags:        []string{"任务执行"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAExecuteTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/execute/command",
		OperationID: "apiExecuteCommand",
		Summary:     "执行命令",
		Description: "在服务器上执行一条命令",
		Tags:        []string{"任务执行"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAExecuteCommand)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/execute/results",
		OperationID: "apiGetLastResults",
		Summary:     "获取最新执行结果",
		Description: "获取最近的任务执行结果",
		Tags:        []string{"任务执行"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ec.TAGetLastResults)
}

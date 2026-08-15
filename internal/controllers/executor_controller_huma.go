package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// OpenAPI (Bearer Token) 接口 —— 任务执行
// ===========================================================================

// OAExecuteTaskInput 运行任务（OpenAPI）
type OAExecuteTaskInput struct {
	ID   string              `path:"id" description:"任务ID"`
	Body vo.ExecuteTaskReq   // body 可选，无必填字段
}

// OAExecuteTaskOutput 运行任务（OpenAPI）
type OAExecuteTaskOutput struct {
	Body utils.HumaResponse[*vo.ExecutionResultVO]
}

// OAExecuteTask 运行任务
func (ec *ExecutorController) OAExecuteTask(ctx context.Context, input *OAExecuteTaskInput) (*OAExecuteTaskOutput, error) {
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

	return &OAExecuteTaskOutput{
		Body: utils.HumaResponse[*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVO(result),
		},
	}, nil
}

// OAGetLastResultsInput 获取最新执行结果（OpenAPI）
type OAGetLastResultsInput struct {
	Count int `query:"count" default:"10" description:"数量"`
}

// OAGetLastResultsOutput 获取最新执行结果（OpenAPI）
type OAGetLastResultsOutput struct {
	Body utils.HumaResponse[[]*vo.ExecutionResultVO]
}

// OAGetLastResults 获取最新执行结果
func (ec *ExecutorController) OAGetLastResults(ctx context.Context, input *OAGetLastResultsInput) (*OAGetLastResultsOutput, error) {
	count := input.Count
	if count <= 0 {
		count = 10
	}

	results := ec.executorService.GetLastResults(count)

	return &OAGetLastResultsOutput{
		Body: utils.HumaResponse[[]*vo.ExecutionResultVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToExecutionResultVOList(results),
		},
	}, nil
}

// RegisterOpenAPIExecutorRoutes 注册 OpenAPI 任务执行相关 Huma 路由
func (ec *ExecutorController) RegisterOpenAPIExecutorRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/execute/task/{id}",
		OperationID: "openapiExecuteTask",
		Summary:     "运行任务",
		Description: "立即执行指定的任务",
		Tags:        []string{"任务执行"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAExecuteTask)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/execute/results",
		OperationID: "openapiGetLastResults",
		Summary:     "获取最新执行结果",
		Description: "获取最新任务或命令执行的结果列表",
		Tags:        []string{"任务执行"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, ec.OAGetLastResults)
}

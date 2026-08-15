package controllers

import (
	"context"
	"net/http"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 终端
// ===========================================================================

// TATermGetCommandsOutput 获取所有可用命令
type TATermGetCommandsOutput struct {
	Body utils.HumaResponse[[]map[string]string]
}

// TATermGetCommands 获取所有可用的 cmd 列表及说明
func (tc *TerminalController) TATermGetCommands(ctx context.Context, input *struct{}) (*TATermGetCommandsOutput, error) {
	var cmds []map[string]string
	for _, cmdInfo := range constant.Commands {
		cmds = append(cmds, map[string]string{
			"name":        cmdInfo.Name,
			"description": cmdInfo.Description,
		})
	}

	return &TATermGetCommandsOutput{
		Body: utils.HumaResponse[[]map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: cmds,
		},
	}, nil
}

// TATermExecuteCommandInput 执行单个命令
type TATermExecuteCommandInput struct {
	Body struct {
		Command string `json:"command" description:"要执行的命令"`
	}
}

// TATermExecuteCommandOutput 执行单个命令
type TATermExecuteCommandOutput struct {
	Body utils.HumaResponse[struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}]
}

// TATermExecuteCommand 执行单个命令并返回结果
func (tc *TerminalController) TATermExecuteCommand(ctx context.Context, input *TATermExecuteCommandInput) (*TATermExecuteCommandOutput, error) {
	if constant.DemoMode {
		return nil, utils.HumaBadRequest("演示模式下不能执行命令")
	}

	if input.Body.Command == "" {
		return nil, utils.HumaBadRequest("命令不能为空")
	}

	cmd := utils.NewShellCommandCmd(input.Body.Command)
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}
	if userID == "" {
		userID = "1" // 与 WebSocket 终端保持一致，保留原有兜底行为
	}
	cmd.Env = tc.buildTerminalEnv(userID)
	output, err := cmd.CombinedOutput()

	data := struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}{
		Output: string(output),
	}
	if err != nil {
		data.Error = err.Error()
	}

	return &TATermExecuteCommandOutput{
		Body: utils.HumaResponse[struct {
			Output string `json:"output"`
			Error  string `json:"error,omitempty"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: data,
		},
	}, nil
}

// RegisterAPITerminalRoutes 注册 /api/v1 终端 Huma 路由
func (tc *TerminalController) RegisterAPITerminalRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/terminal/cmds",
		OperationID: "apiTermGetCommands",
		Summary:     "获取所有可用命令",
		Description: "获取终端中可用的内置命令列表及说明",
		Tags:        []string{"终端"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TATermGetCommands)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/terminal/execute",
		OperationID: "apiTermExecuteCommand",
		Summary:     "执行单个命令",
		Description: "在服务器上执行一条命令并返回结果",
		Tags:        []string{"终端"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, tc.TATermExecuteCommand)
}

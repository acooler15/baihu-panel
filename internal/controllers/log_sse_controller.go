package controllers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
)

type LogSSEController struct{}

func NewLogSSEController() *LogSSEController {
	return &LogSSEController{}
}

// LogEventVO SSE 推送的日志条目（data 字段），格式与原 gin 实现的 gin.H{"text": ...} 保持一致。
type LogEventVO struct {
	Text string `json:"text"`
}

// TALogStreamInput 日志流输入
type TALogStreamInput struct {
	LogID string `query:"log_id" required:"true" description:"任务日志 ID"`
}

// StreamLogSSE 实时日志流（SSE）。推送 `message` 事件，data 为 LogEventVO。
func (lc *LogSSEController) StreamLogSSE(ctx context.Context, input *TALogStreamInput, send sse.Sender) {
	logID := input.LogID

	// 1. 检查数据库中是否已结束
	var taskLog models.TaskLog
	res := database.DB.Where("id = ?", logID).Limit(1).Find(&taskLog)
	if res.Error == nil && res.RowsAffected > 0 {
		if taskLog.Status != "running" {
			// 已结束，读取库内日志
			content, err := utils.DecompressFromBase64(string(taskLog.Output))
			if err != nil {
				_ = send.Data(LogEventVO{Text: "解压日志失败: " + err.Error()})
				return
			}
			_ = send.Data(LogEventVO{Text: content})
			return
		}
	}

	// 2. 未结束或未找到记录，尝试从 TinyLogManager 获取
	tl := tasks.GetActiveLog(logID)
	if tl == nil {
		_ = send.Data(LogEventVO{Text: "未找到正在运行的任务日志"})
		return
	}

	// 发送系统提示
	_ = send.Data(LogEventVO{Text: fmt.Sprintf("[System] 连接成功，正在监听日志... (LogID: %s)\n", logID)})

	// 发送最后 100 行
	lastLines, err := tl.ReadLastLines(100)
	if err == nil && len(lastLines) > 0 {
		_ = send.Data(LogEventVO{Text: string(lastLines)})
	}

	// 订阅实时更新
	sub := tl.Subscribe()
	defer tl.Unsubscribe(sub)

	for {
		select {
		case data, ok := <-sub:
			if !ok {
				// 任务结束，尝试刷新最后一次库内完整内容
				var finalLog models.TaskLog
				r := database.DB.Where("id = ?", logID).Limit(1).Find(&finalLog)
				if r.Error == nil && r.RowsAffected > 0 {
					content, _ := utils.DecompressFromBase64(string(finalLog.Output))
					if content != "" {
						_ = send.Data(LogEventVO{Text: "\n--- 任务已结束 ---\n"})
					}
				}
				return
			}
			if err := send.Data(LogEventVO{Text: string(data)}); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// RegisterAPILogSSERoutes 注册 /api/v1 日志 SSE 路由
func (lc *LogSSEController) RegisterAPILogSSERoutes(api huma.API) {
	sse.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/logs/sse",
		OperationID: "streamLogs",
		Summary:     "实时日志流（SSE）",
		Description: "通过 Server-Sent Events 实时推送任务执行日志。连接建立后持续推送 `message` 事件，内容为 `{code, data, msg}`。客户端断开或出错时连接自动关闭。",
		Tags:        []string{"日志"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, map[string]any{
		"message": LogEventVO{},
	}, lc.StreamLogSSE)
}

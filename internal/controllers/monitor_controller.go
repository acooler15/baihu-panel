package controllers

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
)

type MonitorController struct {
	executorService *tasks.ExecutorService
}

func NewMonitorController(executorService *tasks.ExecutorService) *MonitorController {
	return &MonitorController{
		executorService: executorService,
	}
}

// ===========================================================================
// 系统监控业务方法
// ===========================================================================

// MonitorEnvVO 环境信息
type MonitorEnvVO struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	GoVersion  string `json:"go_version"`
	NumCPU     int    `json:"num_cpu"`
	Goroutines int    `json:"goroutines"`
}

// MonitorHostVO 主机信息
type MonitorHostVO struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskPercent float64 `json:"disk_percent"`
	Uptime      uint64  `json:"uptime"`
	Platform    string  `json:"platform"`
}

// MonitorMemVO 内存信息
type MonitorMemVO struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"total_alloc"`
	Sys        uint64 `json:"sys"`
	Lookups    uint64 `json:"lookups"`
	Mallocs    uint64 `json:"mallocs"`
	Frees      uint64 `json:"frees"`
}

// MonitorHeapVO 堆信息
type MonitorHeapVO struct {
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
}

// MonitorGCVO GC 信息
type MonitorGCVO struct {
	NextGC       uint64 `json:"next_gc"`
	LastGC       uint64 `json:"last_gc"`
	PauseTotalNs uint64 `json:"pause_total_ns"`
	NumGC        uint32 `json:"num_gc"`
}

// MonitorSchedulerVO 调度器信息
type MonitorSchedulerVO struct {
	Scheduled   int         `json:"scheduled"`
	Running     int         `json:"running"`
	QueueSize   int         `json:"queue_size"`
	WorkerCount int         `json:"worker_count"`
	Workers     interface{} `json:"workers"`
}

// MonitorDataVO 系统监控数据
type MonitorDataVO struct {
	Env       MonitorEnvVO       `json:"env"`
	Host      MonitorHostVO      `json:"host"`
	Mem       MonitorMemVO       `json:"mem"`
	Heap      MonitorHeapVO      `json:"heap"`
	GC        MonitorGCVO        `json:"gc"`
	Scheduler MonitorSchedulerVO `json:"scheduler"`
}

// buildMonitorData 采集系统监控数据
func (mc *MonitorController) buildMonitorData() MonitorDataVO {
	rt := services.GetMonitorService().GetRuntimeMetrics()
	m := rt.MemStats
	metrics := services.GetMonitorService().GetHostMetrics()

	return MonitorDataVO{
		Env: MonitorEnvVO{
			OS:         runtime.GOOS,
			Arch:       runtime.GOARCH,
			GoVersion:  runtime.Version(),
			NumCPU:     runtime.NumCPU(),
			Goroutines: rt.NumGoroutine,
		},
		Host: MonitorHostVO{
			CPUPercent:  metrics.CPUPercent,
			MemTotal:    metrics.VMem.Total,
			MemUsed:     metrics.VMem.Used,
			MemPercent:  metrics.VMem.UsedPercent,
			DiskTotal:   metrics.DiskUsage.Total,
			DiskUsed:    metrics.DiskUsage.Used,
			DiskPercent: metrics.DiskUsage.UsedPercent,
			Uptime:      metrics.HostInfo.Uptime,
			Platform:    metrics.HostInfo.Platform + " " + metrics.HostInfo.PlatformVersion,
		},
		Mem: MonitorMemVO{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			Lookups:    m.Lookups,
			Mallocs:    m.Mallocs,
			Frees:      m.Frees,
		},
		Heap: MonitorHeapVO{
			HeapAlloc:    m.HeapAlloc,
			HeapSys:      m.HeapSys,
			HeapIdle:     m.HeapIdle,
			HeapInuse:    m.HeapInuse,
			HeapReleased: m.HeapReleased,
			HeapObjects:  m.HeapObjects,
		},
		GC: MonitorGCVO{
			NextGC:       m.NextGC,
			LastGC:       m.LastGC,
			PauseTotalNs: m.PauseTotalNs,
			NumGC:        m.NumGC,
		},
		Scheduler: MonitorSchedulerVO{
			Scheduled:   mc.executorService.GetScheduledCount(),
			Running:     mc.executorService.GetRunningCount(),
			QueueSize:   mc.executorService.GetScheduler().GetQueueSize(),
			WorkerCount: mc.executorService.GetScheduler().GetConfig().WorkerCount,
			Workers:     mc.executorService.GetScheduler().GetWorkerStatuses(),
		},
	}
}

// GetSystemMonitorOutput 获取系统监控信息
type GetSystemMonitorOutput struct {
	Body utils.HumaResponse[MonitorDataVO]
}

// GetSystemMonitor 获取系统和内存监控信息
func (mc *MonitorController) GetSystemMonitor(ctx context.Context, input *struct{}) (*GetSystemMonitorOutput, error) {
	data := mc.buildMonitorData()

	return &GetSystemMonitorOutput{
		Body: utils.HumaResponse[MonitorDataVO]{
			Code: 200,
			Msg:  "success",
			Data: data,
		},
	}, nil
}

// MonitorSSEOutput SSE 事件 data，保持原 gin 实现的 {code, data, msg} 格式。
type MonitorSSEOutput struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data MonitorDataVO `json:"data"`
}

// StreamMonitorSSE 实时系统监控数据流（SSE）。每 5 秒推送一次 `message` 事件。
func (mc *MonitorController) StreamMonitorSSE(ctx context.Context, input *struct{}, send sse.Sender) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 初始发送一次数据
	if err := send.Data(MonitorSSEOutput{
		Code: 200,
		Msg:  "success",
		Data: mc.buildMonitorData(),
	}); err != nil {
		return
	}

	for {
		select {
		case <-ticker.C:
			if err := send.Data(MonitorSSEOutput{
				Code: 200,
				Msg:  "success",
				Data: mc.buildMonitorData(),
			}); err != nil {
				return // 客户端断开连接或发送失败
			}
		case <-ctx.Done():
			return // 连接已断开，立即退出
		}
	}
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIMonitorRoutes 注册 /api/v1 系统监控 Huma 路由
func (mc *MonitorController) RegisterAPIMonitorRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"系统监控"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/monitor", OperationID: "GetSystemMonitor", Summary: "获取系统监控信息", Description: "获取系统和内存监控信息", Tags: tag, Security: security}, mc.GetSystemMonitor)

	// SSE 实时监控数据流
	sse.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/monitor/sse",
		OperationID: "monitorSSE",
		Summary:     "系统监控数据流（SSE）",
		Description: "通过 Server-Sent Events 每 5 秒推送一次系统监控数据（CPU/内存/磁盘/调度器状态）。事件格式为 `{code, data, msg}`。",
		Tags:        tag,
		Security:    security,
	}, map[string]any{
		"message": MonitorSSEOutput{},
	}, mc.StreamMonitorSSE)
}

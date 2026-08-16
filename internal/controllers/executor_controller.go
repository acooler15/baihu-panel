package controllers

import (
	"net/http"

	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type ExecutorController struct {
	executorService *tasks.ExecutorService
}

func NewExecutorController(executorService *tasks.ExecutorService) *ExecutorController {
	return &ExecutorController{executorService: executorService}
}

// ExecuteTask 运行任务
func (ec *ExecutorController) ExecuteTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "无效的任务ID"})
		return
	}

	var req struct {
		Envs map[string]string `json:"envs"`
	}
	// 尝试绑定 JSON 体，但不强制要求
	_ = c.ShouldBindJSON(&req)

	var extraEnvs []string
	if req.Envs != nil {
		for k, v := range req.Envs {
			extraEnvs = append(extraEnvs, k+"="+v)
		}
	}

	result := ec.executorService.ExecuteTask(id, extraEnvs)
	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "success", Data: vo.ToExecutionResultVO(result)})
}

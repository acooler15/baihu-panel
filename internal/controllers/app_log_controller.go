package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
)

type AppLogController struct {
	appLogService *services.AppLogService
}

func NewAppLogController() *AppLogController {
	return &AppLogController{
		appLogService: services.NewAppLogService(),
	}
}

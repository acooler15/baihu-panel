package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
)

type DataController struct {
	dataService    *services.DataService
	taskController *TaskController
	envController  *EnvController
}

func NewDataController(tc *TaskController, ec *EnvController) *DataController {
	return &DataController{
		dataService:    services.NewDataService(),
		taskController: tc,
		envController:  ec,
	}
}

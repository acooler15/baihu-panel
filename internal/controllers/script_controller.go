package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
)

type ScriptController struct {
	scriptService *services.ScriptService
}

func NewScriptController(scriptService *services.ScriptService) *ScriptController {
	return &ScriptController{scriptService: scriptService}
}

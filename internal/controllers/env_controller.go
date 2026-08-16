package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
)

type EnvController struct {
	envService *services.EnvService
}

func NewEnvController(envService *services.EnvService) *EnvController {
	return &EnvController{envService: envService}
}

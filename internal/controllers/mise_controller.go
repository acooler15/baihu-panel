package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
)

type MiseController struct {
	service *services.MiseService
}

func NewMiseController(service *services.MiseService) *MiseController {
	return &MiseController{
		service: service,
	}
}

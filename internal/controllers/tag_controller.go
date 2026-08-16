package controllers

import (
	"github.com/engigu/baihu-panel/internal/services"
)

type TagController struct {
	tagService *services.TagService
}

func NewTagController(tagService *services.TagService) *TagController {
	return &TagController{
		tagService: tagService,
	}
}

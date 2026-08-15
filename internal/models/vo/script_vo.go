package vo

import (
	"github.com/engigu/baihu-panel/internal/models"
)

// ScriptCreateReq 脚本创建请求
type ScriptCreateReq struct {
	Name    string `json:"name" binding:"required" description:"脚本名称"`
	Content string `json:"content" binding:"required" description:"脚本内容"`
}

// ScriptUpdateReq 脚本更新请求
type ScriptUpdateReq struct {
	Name    string `json:"name" description:"脚本名称"`
	Content string `json:"content" description:"脚本内容"`
}

// ScriptVO 脚本视图对象
type ScriptVO struct {
	ID        string           `json:"id" description:"脚本ID"`
	Name      string           `json:"name" description:"脚本名称"`
	Content   string           `json:"content,omitempty" description:"脚本内容（列表不返回）"`
	CreatedAt models.LocalTime `json:"created_at" description:"创建时间"`
	UpdatedAt models.LocalTime `json:"updated_at" description:"更新时间"`
}

// ToScriptVO 将 Script 模型转换为 ScriptVO
func ToScriptVO(script *models.Script) *ScriptVO {
	if script == nil {
		return nil
	}
	return &ScriptVO{
		ID:        script.ID,
		Name:      script.Name,
		Content:   string(script.Content),
		CreatedAt: script.CreatedAt,
		UpdatedAt: script.UpdatedAt,
	}
}

// ToScriptVOListFromModels 将 Script 模型列表转换为 ScriptVO 列表
func ToScriptVOListFromModels(scripts []models.Script) []*ScriptVO {
	vos := make([]*ScriptVO, len(scripts))
	for i := range scripts {
		vos[i] = ToScriptVO(&scripts[i])
	}
	return vos
}

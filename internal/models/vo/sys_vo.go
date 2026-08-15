package vo

import (
	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"
)

// UserVO 用户视图对象
type UserVO struct {
	ID        string           `json:"id"`
	Username  string           `json:"username"`
	Email     string           `json:"email"`
	Role      string           `json:"role"`
	CreatedAt models.LocalTime `json:"created_at"`
	UpdatedAt models.LocalTime `json:"updated_at"`
}

// ToUserVO 将 User 模型转换为 UserVO
func ToUserVO(user *models.User) *UserVO {
	if user == nil {
		return nil
	}
	return &UserVO{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// EnvCreateReq 环境变量创建请求
type EnvCreateReq struct {
	Name    string `json:"name" binding:"required" description:"变量名"`
	Value   string `json:"value" binding:"required" description:"变量值"`
	Remark  string `json:"remark" description:"备注"`
	Type    string `json:"type" description:"类型，默认 normal"`
	Hidden  *bool  `json:"hidden" description:"是否隐藏，默认 true"`
	Enabled *bool  `json:"enabled" description:"是否启用，默认 true"`
	Tags    string `json:"tags" description:"标签"`
}

// EnvUpdateReq 环境变量更新请求
type EnvUpdateReq struct {
	Name    string `json:"name" description:"变量名"`
	Value   string `json:"value" description:"变量值"`
	Remark  string `json:"remark" description:"备注"`
	Type    string `json:"type" description:"类型"`
	Hidden  *bool  `json:"hidden" description:"是否隐藏"`
	Enabled *bool  `json:"enabled" description:"是否启用"`
	Tags    string `json:"tags" description:"标签"`
}

// EnvVO 环境变量视图对象
type EnvVO struct {
	ID        string           `json:"id" description:"环境变量ID"`
	Name      string           `json:"name" description:"变量名"`
	Value     string           `json:"value" description:"变量值（机密类型隐藏）"`
	Remark    string           `json:"remark" description:"备注"`
	Type      string           `json:"type" description:"类型"`
	Tags      string           `json:"tags" description:"标签"`
	Hidden    bool             `json:"hidden" description:"是否隐藏"`
	Enabled   bool             `json:"enabled" description:"是否启用"`
	CreatedAt models.LocalTime `json:"created_at" description:"创建时间"`
	UpdatedAt models.LocalTime `json:"updated_at" description:"更新时间"`
}

// ToEnvVO 将 Env 模型转换为 EnvVO
func ToEnvVO(env *models.EnvironmentVariable) *EnvVO {
	if env == nil {
		return nil
	}
	val := string(env.Value)
	if env.Type == constant.EnvTypeSecret {
		val = "********"
	}
	return &EnvVO{
		ID:        env.ID,
		Name:      env.Name,
		Value:     val,
		Remark:    env.Remark,
		Type:      env.Type,
		Tags:      env.Tags,
		Hidden:    utils.DerefBool(env.Hidden, true),
		Enabled:   utils.DerefBool(env.Enabled, true),
		CreatedAt: env.CreatedAt,
		UpdatedAt: env.UpdatedAt,
	}
}

// ToEnvVOList 将 Env 模型列表转换为 EnvVO 列表
func ToEnvVOList(envs []*models.EnvironmentVariable) []*EnvVO {
	if envs == nil {
		return nil
	}
	vos := make([]*EnvVO, len(envs))
	for i, e := range envs {
		vos[i] = ToEnvVO(e)
	}
	return vos
}

// ToEnvVOListFromModels 将 Env 模型列表转换为 EnvVO 列表
func ToEnvVOListFromModels(envs []models.EnvironmentVariable) []*EnvVO {
	vos := make([]*EnvVO, len(envs))
	for i := range envs {
		vos[i] = ToEnvVO(&envs[i])
	}
	return vos
}

// LoginLogVO 登录日志视图对象
type LoginLogVO struct {
	ID        string           `json:"id"`
	Username  string           `json:"username"`
	IP        string           `json:"ip"`
	UserAgent string           `json:"user_agent"`
	Status    string           `json:"status"`
	Message   string           `json:"message"`
	CreatedAt models.LocalTime `json:"created_at"`
}

// TokenConfig Token 配置结构体
type TokenConfig struct {
	Enabled  bool   `json:"enabled"`
	Token    string `json:"token"`
	ExpireAt string `json:"expire_at"`
}

package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/eventbus"
	"github.com/engigu/baihu-panel/internal/middleware"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/pquerna/otp/totp"
)

type AuthController struct {
	userService     *services.UserService
	settingsService *services.SettingsService
	loginLogService *services.LoginLogService
}

type loginAttempt struct {
	Count       int
	LastAttempt time.Time
}

var loginAttempts sync.Map

func init() {
	// 定期清理过期的登录尝试统计，防止内存溢出
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		for range ticker.C {
			loginAttempts.Range(func(key, value any) bool {
				attempt := value.(*loginAttempt)
				if time.Since(attempt.LastAttempt) > 10*time.Minute {
					loginAttempts.Delete(key)
				}
				return true
			})
		}
	}()
}

func NewAuthController(userService *services.UserService, settingsService *services.SettingsService, loginLogService *services.LoginLogService) *AuthController {
	return &AuthController{
		userService:     userService,
		settingsService: settingsService,
		loginLogService: loginLogService,
	}
}



// ===========================================================================
// /api/v1 Auth 普通用户接口 —— Huma handler（阶段 6）
// 鉴权由 newHuma 的 selector 为 /auth/me、/auth/otp/* 套用 AuthRequired。
// ===========================================================================

// AGGetCurrentUserOutput 获取当前用户信息
type AGGetCurrentUserOutput struct {
	Body utils.HumaResponse[vo.CurrentUserVO]
}

// AGGetCurrentUser 获取当前登录用户信息（普通用户可访问）
func (ac *AuthController) AGGetCurrentUser(ctx context.Context, input *struct{}) (*AGGetCurrentUserOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}
	user, err := ac.userService.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, utils.HumaUnauthorized("会话无效")
	}
	return &AGGetCurrentUserOutput{
		Body: utils.HumaResponse[vo.CurrentUserVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.CurrentUserVO{
				Username: user.Username,
				Role:     user.Role,
			},
		},
	}, nil
}

// AGGetOTPStatusOutput 获取 OTP 状态
type AGGetOTPStatusOutput struct {
	Body utils.HumaResponse[vo.OTPStatusVO]
}

// AGGetOTPStatus 获取两步验证状态（普通用户可访问）
func (ac *AuthController) AGGetOTPStatus(ctx context.Context, input *struct{}) (*AGGetOTPStatusOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}
	user, err := ac.userService.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, utils.HumaUnauthorized("用户不存在")
	}
	return &AGGetOTPStatusOutput{
		Body: utils.HumaResponse[vo.OTPStatusVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.OTPStatusVO{
				OTPEnabled: user.OtpEnabled,
			},
		},
	}, nil
}

// AGGenerateOTPOutput 生成 OTP 密钥
type AGGenerateOTPOutput struct {
	Body utils.HumaResponse[vo.OTPGenerateVO]
}

// AGGenerateOTP 生成新的 TOTP 密钥与二维码内容（普通用户可访问）
func (ac *AuthController) AGGenerateOTP(ctx context.Context, input *struct{}) (*AGGenerateOTPOutput, error) {
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}
	user, err := ac.userService.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, utils.HumaUnauthorized("用户不存在")
	}

	// 从设置服务获取站点标题作为账号名称后缀，以便于区分多环境
	siteTitle := ac.settingsService.Get(constant.SectionSite, constant.KeyTitle)
	accountName := user.Username
	if siteTitle != "" {
		accountName = fmt.Sprintf("%s@%s", user.Username, siteTitle)
	}

	// 固定 Issuer 为 "BaihuPanel" 以保留 Authenticator App 内置的 Logo 识别
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "BaihuPanel",
		AccountName: accountName,
	})
	if err != nil {
		return nil, utils.HumaServerError("生成密钥失败")
	}

	return &AGGenerateOTPOutput{
		Body: utils.HumaResponse[vo.OTPGenerateVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.OTPGenerateVO{
				Secret: key.Secret(),
				URL:    key.URL(),
			},
		},
	}, nil
}

// AGEnableOTPInput 开启 OTP 请求
type AGEnableOTPInput struct {
	Body struct {
		Secret string `json:"secret" description:"TOTP 密钥（Base32）"`
		Code   string `json:"code" description:"一次性验证码"`
	}
}

// AGEnableOTPOutput 开启 OTP 结果
type AGEnableOTPOutput struct {
	Body utils.HumaResponse[any]
}

// AGEnableOTP 开启两步验证（普通用户可访问）
func (ac *AuthController) AGEnableOTP(ctx context.Context, input *AGEnableOTPInput) (*AGEnableOTPOutput, error) {
	req := input.Body
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	// 验证验证码是否与此 secret 匹配，防止错误绑定导致锁在外面
	if !totp.Validate(req.Code, req.Secret) {
		return nil, utils.HumaBadRequest("验证码校验失败")
	}

	// 绑定并保存
	if err := ac.userService.UpdateOTP(userID, req.Secret, true); err != nil {
		return nil, utils.HumaServerError("开启两步验证失败")
	}

	return &AGEnableOTPOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "开启两步验证成功",
		},
	}, nil
}

// AGDisableOTPInput 关闭 OTP 请求
type AGDisableOTPInput struct {
	Body struct {
		Code string `json:"code" description:"一次性验证码"`
	}
}

// AGDisableOTPOutput 关闭 OTP 结果
type AGDisableOTPOutput struct {
	Body utils.HumaResponse[any]
}

// AGDisableOTP 关闭两步验证（普通用户可访问）
func (ac *AuthController) AGDisableOTP(ctx context.Context, input *AGDisableOTPInput) (*AGDisableOTPOutput, error) {
	req := input.Body
	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}

	user, err := ac.userService.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, utils.HumaUnauthorized("用户不存在")
	}

	if !user.OtpEnabled {
		return nil, utils.HumaBadRequest("两步验证尚未开启")
	}

	// 验证验证码
	if !totp.Validate(req.Code, user.OtpSecret) {
		return nil, utils.HumaBadRequest("验证码错误")
	}

	// 禁用并清除 secret
	if err := ac.userService.UpdateOTP(userID, "", false); err != nil {
		return nil, utils.HumaServerError("关闭两步验证失败")
	}

	return &AGDisableOTPOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "关闭两步验证成功",
		},
	}, nil
}

// RegisterAPIAuthRoutes 注册 /api/v1 Auth 普通用户接口 Huma 路由（阶段 6）。
// 鉴权由 newHuma 的 selector 按路径套用 AuthRequired。
func (ac *AuthController) RegisterAPIAuthRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/auth/me",
		OperationID: "apiGetCurrentUser",
		Summary:     "获取当前用户",
		Description: "获取当前登录用户的用户名与角色",
		Tags:        []string{"认证"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.AGGetCurrentUser)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/auth/otp/status",
		OperationID: "apiGetOTPStatus",
		Summary:     "获取两步验证状态",
		Description: "获取当前用户是否已开启两步验证",
		Tags:        []string{"认证"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.AGGetOTPStatus)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/auth/otp/generate",
		OperationID: "apiGenerateOTP",
		Summary:     "生成两步验证密钥",
		Description: "生成新的 TOTP 密钥与二维码内容",
		Tags:        []string{"认证"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.AGGenerateOTP)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/auth/otp/enable",
		OperationID: "apiEnableOTP",
		Summary:     "开启两步验证",
		Description: "校验验证码后开启两步验证",
		Tags:        []string{"认证"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.AGEnableOTP)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/auth/otp/disable",
		OperationID: "apiDisableOTP",
		Summary:     "关闭两步验证",
		Description: "校验验证码后关闭两步验证",
		Tags:        []string{"认证"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, ac.AGDisableOTP)

	// 公开接口（无需鉴权，selector 中按路径放行）
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/auth/login",
		OperationID: "apiLogin",
		Summary:     "用户登录",
		Description: "用户名密码登录。若开启了两步验证，返回 require_otp 与 otp_pending_token，需再调用 /auth/login/otp 完成登录。",
		Tags:        []string{"认证"},
	}, ac.LoginHuma)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/auth/login/otp",
		OperationID: "apiLoginOTP",
		Summary:     "两步验证登录",
		Description: "使用临时凭证与一次性验证码完成两步验证登录。",
		Tags:        []string{"认证"},
	}, ac.VerifyOTPHuma)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/auth/logout",
		OperationID: "apiLogout",
		Summary:     "退出登录",
		Description: "使当前会话失效并清除认证 Cookie。",
		Tags:        []string{"认证"},
	}, ac.LogoutHuma)
}

// ===========================================================================
// 认证公开接口（Huma，迁移自 Gin 原生 Login/VerifyOTP/Logout）
// ===========================================================================

// LoginHumaInput 登录请求
type LoginHumaInput struct {
	Body struct {
		Username string `json:"username" required:"true" description:"用户名"`
		Password string `json:"password" required:"true" description:"密码"`
	}
}

// LoginHumaOutput 登录结果
type LoginHumaOutput struct {
	Body utils.HumaResponse[struct {
		User            string `json:"user,omitempty"`
		RequireOTP      bool   `json:"require_otp,omitempty"`
		OtpPendingToken string `json:"otp_pending_token,omitempty"`
	}]
}

// LoginHuma 用户名密码登录（公开接口）
func (ac *AuthController) LoginHuma(ctx context.Context, input *LoginHumaInput) (*LoginHumaOutput, error) {
	req := input.Body
	gc := utils.GetGinContext(ctx)

	ip := ""
	userAgent := ""
	if gc != nil {
		ip = gc.ClientIP()
		userAgent = gc.GetHeader("User-Agent")
	}

	// 暴力破解防御
	if val, ok := loginAttempts.Load(ip); ok {
		attempt := val.(*loginAttempt)
		if attempt.Count >= 5 && time.Since(attempt.LastAttempt) < time.Minute {
			eventbus.DefaultBus.Publish(eventbus.Event{
				Type: constant.EventBruteForceLogin,
				Payload: map[string]interface{}{
					"ip":        ip,
					"username":  req.Username,
					"userAgent": userAgent,
				},
			})
			return nil, utils.HumaTooManyRequests("尝试次数过多，请一分钟后再试")
		}
		// 如果距离上次尝试已超过一分钟，重置计数
		if time.Since(attempt.LastAttempt) >= time.Minute {
			loginAttempts.Delete(ip)
		}
	}

	user := ac.userService.GetUserByUsername(req.Username)
	if user == nil || !ac.userService.ValidatePassword(user, req.Password) {
		// 记录失败尝试
		val, _ := loginAttempts.LoadOrStore(ip, &loginAttempt{Count: 0, LastAttempt: time.Now()})
		attempt := val.(*loginAttempt)
		attempt.Count++
		attempt.LastAttempt = time.Now()

		// 记录登录失败日志
		eventbus.DefaultBus.Publish(eventbus.Event{
			Type: constant.EventUserLogin,
			Payload: map[string]interface{}{
				"ip":        ip,
				"username":  req.Username,
				"userAgent": userAgent,
				"status":    "failed",
				"message":   "用户名或密码错误",
			},
		})
		return nil, utils.HumaUnauthorized("用户名或密码错误")
	}

	// 登录成功，清除尝试记录
	loginAttempts.Delete(ip)

	// 检查是否开启了两步验证
	if user.OtpEnabled {
		// 生成临时待验证 OTP 的 token，有效期 5 分钟
		pendingToken, err := utils.GenerateOtpPendingToken(user.ID, constant.Secret)
		if err != nil {
			return nil, utils.HumaServerError("生成临时凭证失败")
		}
		return &LoginHumaOutput{
			Body: utils.HumaResponse[struct {
				User            string `json:"user,omitempty"`
				RequireOTP      bool   `json:"require_otp,omitempty"`
				OtpPendingToken string `json:"otp_pending_token,omitempty"`
			}]{
				Code: 200,
				Msg:  "success",
				Data: struct {
					User            string `json:"user,omitempty"`
					RequireOTP      bool   `json:"require_otp,omitempty"`
					OtpPendingToken string `json:"otp_pending_token,omitempty"`
				}{
					RequireOTP:      true,
					OtpPendingToken: pendingToken,
				},
			},
		}, nil
	}

	expireDays := 7
	if days := ac.settingsService.Get(constant.SectionSite, constant.KeyCookieDays); days != "" {
		if d, err := strconv.Atoi(days); err == nil && d > 0 {
			expireDays = d
		}
	}

	// 生成 token
	token, err := utils.GenerateToken(user.ID, user.Username, user.TokenVersion, expireDays, constant.Secret)
	if err != nil {
		eventbus.DefaultBus.Publish(eventbus.Event{
			Type: constant.EventUserLogin,
			Payload: map[string]interface{}{
				"ip":        ip,
				"username":  req.Username,
				"userAgent": userAgent,
				"status":    "failed",
				"message":   "Token生成失败",
			},
		})
		return nil, utils.HumaServerError("登录失败")
	}

	// 设置 Cookie
	if gc != nil {
		middleware.SetAuthCookie(gc, token, expireDays)
	}

	// 记录登录成功日志
	eventbus.DefaultBus.Publish(eventbus.Event{
		Type: constant.EventUserLogin,
		Payload: map[string]interface{}{
			"ip":        ip,
			"username":  req.Username,
			"userAgent": userAgent,
			"status":    "success",
			"message":   "登录成功",
		},
	})

	return &LoginHumaOutput{
		Body: utils.HumaResponse[struct {
			User            string `json:"user,omitempty"`
			RequireOTP      bool   `json:"require_otp,omitempty"`
			OtpPendingToken string `json:"otp_pending_token,omitempty"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				User            string `json:"user,omitempty"`
				RequireOTP      bool   `json:"require_otp,omitempty"`
				OtpPendingToken string `json:"otp_pending_token,omitempty"`
			}{
				User: user.Username,
			},
		},
	}, nil
}

// VerifyOTPHumaInput 两步验证登录请求
type VerifyOTPHumaInput struct {
	Body struct {
		OtpPendingToken string `json:"otp_pending_token" required:"true" description:"临时凭证"`
		Code            string `json:"code" required:"true" description:"一次性验证码"`
	}
}

// VerifyOTPHumaOutput 两步验证登录结果
type VerifyOTPHumaOutput struct {
	Body utils.HumaResponse[struct {
		User string `json:"user,omitempty"`
	}]
}

// VerifyOTPHuma 两步验证登录（公开接口）
func (ac *AuthController) VerifyOTPHuma(ctx context.Context, input *VerifyOTPHumaInput) (*VerifyOTPHumaOutput, error) {
	req := input.Body
	gc := utils.GetGinContext(ctx)

	userID, err := utils.ParseOtpPendingToken(req.OtpPendingToken, constant.Secret)
	if err != nil {
		return nil, utils.HumaUnauthorized("临时凭证无效或已过期")
	}

	user, err := ac.userService.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, utils.HumaUnauthorized("用户不存在")
	}

	if !user.OtpEnabled || user.OtpSecret == "" {
		return nil, utils.HumaBadRequest("未开启两步验证")
	}

	// 验证 OTP 验证码
	if !totp.Validate(req.Code, user.OtpSecret) {
		return nil, utils.HumaUnauthorized("验证码错误")
	}

	// 校验通过，生成正式 Token 并登录
	expireDays := 7
	if days := ac.settingsService.Get(constant.SectionSite, constant.KeyCookieDays); days != "" {
		if d, err := strconv.Atoi(days); err == nil && d > 0 {
			expireDays = d
		}
	}

	token, err := utils.GenerateToken(user.ID, user.Username, user.TokenVersion, expireDays, constant.Secret)
	if err != nil {
		return nil, utils.HumaServerError("登录失败")
	}

	if gc != nil {
		middleware.SetAuthCookie(gc, token, expireDays)
	}

	// 记录登录成功日志
	ip := ""
	userAgent := ""
	if gc != nil {
		ip = gc.ClientIP()
		userAgent = gc.GetHeader("User-Agent")
	}
	eventbus.DefaultBus.Publish(eventbus.Event{
		Type: constant.EventUserLogin,
		Payload: map[string]interface{}{
			"ip":        ip,
			"username":  user.Username,
			"userAgent": userAgent,
			"status":    "success",
			"message":   "两步验证登录成功",
		},
	})

	return &VerifyOTPHumaOutput{
		Body: utils.HumaResponse[struct {
			User string `json:"user,omitempty"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				User string `json:"user,omitempty"`
			}{
				User: user.Username,
			},
		},
	}, nil
}

// LogoutHumaOutput 退出登录结果
type LogoutHumaOutput struct {
	Body utils.HumaResponse[any]
}

// LogoutHuma 退出登录（公开接口）
func (ac *AuthController) LogoutHuma(ctx context.Context, input *struct{}) (*LogoutHumaOutput, error) {
	gc := utils.GetGinContext(ctx)
	if gc != nil {
		if userID, exists := gc.Get("userID"); exists {
			ac.userService.InvalidateUserTokens(userID.(string))
		}
		middleware.ClearAuthCookie(gc)
	}
	return &LogoutHumaOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "退出成功",
		},
	}, nil
}

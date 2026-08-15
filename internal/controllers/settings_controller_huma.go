package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/eventbus"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/tunnel"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v3/process"
)

// ===========================================================================
// /api/v1 管理接口 —— 系统设置
// ===========================================================================

// TAChangePasswordInput 修改密码
type TAChangePasswordInput struct {
	Body struct {
		OldUsername string `json:"old_username" description:"原账号"`
		Username    string `json:"username" description:"新账号"`
		OldPassword string `json:"old_password" description:"原密码"`
		NewPassword string `json:"new_password" description:"新密码"`
	}
}

// TAChangePasswordOutput 修改密码
type TAChangePasswordOutput struct {
	Body utils.HumaResponse[any]
}

// TAChangePassword 修改密码及账号信息
func (sc *SettingsController) TAChangePassword(ctx context.Context, input *TAChangePasswordInput) (*TAChangePasswordOutput, error) {
	if constant.DemoMode {
		return nil, utils.HumaBadRequest("演示模式下不能修改账号或密码")
	}

	req := input.Body

	userID := ""
	if c := utils.GetGinContext(ctx); c != nil {
		userID = c.GetString("userID")
	}
	var user *models.User
	res := database.DB.Where("id = ?", userID).Limit(1).Find(&user)
	if res.Error != nil || res.RowsAffected == 0 {
		return nil, utils.HumaNotFound("用户不存在")
	}

	// 统一校验原账密
	if req.OldUsername != "" && req.OldUsername != user.Username {
		return nil, utils.HumaBadRequest("原账号不正确")
	}

	if !sc.userService.AuthenticateUser(user.Username, req.OldPassword) {
		return nil, utils.HumaBadRequest("原密码错误")
	}

	var updated bool
	var logoutRequired bool

	// 1. 处理用户名修改
	if req.Username != "" && req.Username != user.Username {
		if err := sc.userService.UpdateAccount(user.ID, req.Username); err != nil {
			return nil, utils.HumaBadRequest(err.Error())
		}
		updated = true
		logoutRequired = true
	}

	// 2. 处理密码修改
	if req.NewPassword != "" {
		if len(req.NewPassword) < 6 {
			return nil, utils.HumaBadRequest("新密码至少6位")
		}
		if err := sc.userService.UpdatePassword(user.ID, req.NewPassword); err != nil {
			return nil, utils.HumaServerError("修改密码失败")
		}
		updated = true
		logoutRequired = true
	}

	if !updated {
		return &TAChangePasswordOutput{
			Body: utils.HumaResponse[any]{
				Code: 200,
				Msg:  "未检测到变更内容",
			},
		}, nil
	}

	eventbus.DefaultBus.Publish(eventbus.Event{
		Type: constant.EventPasswordChanged,
		Payload: map[string]interface{}{
			"username": user.Username,
		},
	})

	msg := "保存成功"
	if logoutRequired {
		msg += "，请重新登录"
	}

	return &TAChangePasswordOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  msg,
		},
	}, nil
}

// TAGetSiteSettingsOutput 获取站点设置
type TAGetSiteSettingsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// TAGetSiteSettings 获取站点设置
func (sc *SettingsController) TAGetSiteSettings(ctx context.Context, input *struct{}) (*TAGetSiteSettingsOutput, error) {
	settings := sc.settingsService.GetSection(constant.SectionSite)

	if settings[constant.KeyTitle] == "" {
		settings[constant.KeyTitle] = "白虎面板"
		sc.settingsService.Set(constant.SectionSite, constant.KeyTitle, "白虎面板")
	}
	if settings[constant.KeySubtitle] == "" {
		settings[constant.KeySubtitle] = "极致轻量、高性能的自动化任务调度平台"
		sc.settingsService.Set(constant.SectionSite, constant.KeySubtitle, "极致轻量、高性能的自动化任务调度平台")
	}
	if settings[constant.KeyIcon] == "" {
		settings[constant.KeyIcon] = constant.DefaultIcon
		sc.settingsService.Set(constant.SectionSite, constant.KeyIcon, constant.DefaultIcon)
	}

	if tokenJson, ok := settings[constant.KeyOpenapiToken]; ok && tokenJson != "" {
		var tokenConfig vo.TokenConfig
		if err := json.Unmarshal([]byte(tokenJson), &tokenConfig); err == nil {
			settings["openapi_token"] = tokenConfig.Token
			settings["openapi_token_expire"] = tokenConfig.ExpireAt
			if tokenConfig.Enabled {
				settings["openapi_enabled"] = "true"
			} else {
				settings["openapi_enabled"] = "false"
			}
		}
	}

	settings["system_notice_days"] = sc.settingsService.Get(constant.SectionSystem, constant.KeySystemNoticeDays)
	settings["system_notice_max_count"] = sc.settingsService.Get(constant.SectionSystem, constant.KeySystemNoticeMaxCount)
	settings["push_log_days"] = sc.settingsService.Get(constant.SectionSystem, constant.KeyPushLogDays)
	settings["push_log_max_count"] = sc.settingsService.Get(constant.SectionSystem, constant.KeyPushLogMaxCount)
	settings["login_log_days"] = sc.settingsService.Get(constant.SectionSystem, constant.KeyLoginLogDays)
	settings["login_log_max_count"] = sc.settingsService.Get(constant.SectionSystem, constant.KeyLoginLogMaxCount)
	settings["scheduler_log_days"] = sc.settingsService.Get(constant.SectionSystem, constant.KeySchedulerLogDays)
	settings["scheduler_log_max_count"] = sc.settingsService.Get(constant.SectionSystem, constant.KeySchedulerLogMaxCount)

	return &TAGetSiteSettingsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: settings,
		},
	}, nil
}

// TAUpdateSiteSettingsInput 更新站点设置
type TAUpdateSiteSettingsInput struct {
	Body struct {
		Title                string `json:"title" description:"站点标题"`
		Subtitle             string `json:"subtitle" description:"站点副标题"`
		Icon                 string `json:"icon" description:"图标"`
		PageSize             string `json:"page_size" description:"每页数量"`
		CookieDays           string `json:"cookie_days" description:"Cookie 有效期(天)"`
		OpenapiEnabled       bool   `json:"openapi_enabled" description:"是否启用 OpenAPI"`
		OpenapiToken         string `json:"openapi_token" description:"OpenAPI Token"`
		OpenapiTokenExpire   string `json:"openapi_token_expire" description:"OpenAPI Token 过期时间"`
		SystemNoticeDays     string `json:"system_notice_days" description:"系统通知保留天数"`
		SystemNoticeMaxCount string `json:"system_notice_max_count" description:"系统通知保留条数"`
		PushLogDays          string `json:"push_log_days" description:"推送日志保留天数"`
		PushLogMaxCount      string `json:"push_log_max_count" description:"推送日志保留条数"`
		LoginLogDays         string `json:"login_log_days" description:"登录日志保留天数"`
		LoginLogMaxCount     string `json:"login_log_max_count" description:"登录日志保留条数"`
		SchedulerLogDays     string `json:"scheduler_log_days" description:"调度日志保留天数"`
		SchedulerLogMaxCount string `json:"scheduler_log_max_count" description:"调度日志保留条数"`
	}
}

// TAUpdateSiteSettingsOutput 更新站点设置
type TAUpdateSiteSettingsOutput struct {
	Body utils.HumaResponse[any]
}

// TAUpdateSiteSettings 更新站点设置
func (sc *SettingsController) TAUpdateSiteSettings(ctx context.Context, input *TAUpdateSiteSettingsInput) (*TAUpdateSiteSettingsOutput, error) {
	req := input.Body

	openapiTokenJson := ""
	if req.OpenapiToken != "" || req.OpenapiTokenExpire != "" || req.OpenapiEnabled {
		tokenConfig := vo.TokenConfig{
			Enabled:  req.OpenapiEnabled,
			Token:    req.OpenapiToken,
			ExpireAt: req.OpenapiTokenExpire,
		}
		if b, err := json.Marshal(tokenConfig); err == nil {
			openapiTokenJson = string(b)
		}
	}

	values := map[string]string{
		constant.KeyTitle:        req.Title,
		constant.KeySubtitle:     req.Subtitle,
		constant.KeyIcon:         req.Icon,
		constant.KeyPageSize:     req.PageSize,
		constant.KeyCookieDays:   req.CookieDays,
		constant.KeyOpenapiToken: openapiTokenJson,
	}

	if err := sc.settingsService.SetSection(constant.SectionSite, values); err != nil {
		return nil, utils.HumaServerError("保存失败")
	}

	sc.settingsService.Set(constant.SectionSystem, constant.KeySystemNoticeDays, req.SystemNoticeDays)
	sc.settingsService.Set(constant.SectionSystem, constant.KeySystemNoticeMaxCount, req.SystemNoticeMaxCount)
	sc.settingsService.Set(constant.SectionSystem, constant.KeyPushLogDays, req.PushLogDays)
	sc.settingsService.Set(constant.SectionSystem, constant.KeyPushLogMaxCount, req.PushLogMaxCount)
	sc.settingsService.Set(constant.SectionSystem, constant.KeyLoginLogDays, req.LoginLogDays)
	sc.settingsService.Set(constant.SectionSystem, constant.KeyLoginLogMaxCount, req.LoginLogMaxCount)
	sc.settingsService.Set(constant.SectionSystem, constant.KeySchedulerLogDays, req.SchedulerLogDays)
	sc.settingsService.Set(constant.SectionSystem, constant.KeySchedulerLogMaxCount, req.SchedulerLogMaxCount)

	return &TAUpdateSiteSettingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// TAGenerateOpenapiTokenOutput 生成 OpenAPI Token
type TAGenerateOpenapiTokenOutput struct {
	Body utils.HumaResponse[struct {
		Token string `json:"token"`
	}]
}

// TAGenerateOpenapiToken 生成 OpenAPI Token
func (sc *SettingsController) TAGenerateOpenapiToken(ctx context.Context, input *struct{}) (*TAGenerateOpenapiTokenOutput, error) {
	return &TAGenerateOpenapiTokenOutput{
		Body: utils.HumaResponse[struct {
			Token string `json:"token"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Token string `json:"token"`
			}{Token: strings.ToLower(utils.RandomString(32))},
		},
	}, nil
}

// TAGetSchedulerSettingsOutput 获取调度设置
type TAGetSchedulerSettingsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// TAGetSchedulerSettings 获取调度设置
func (sc *SettingsController) TAGetSchedulerSettings(ctx context.Context, input *struct{}) (*TAGetSchedulerSettingsOutput, error) {
	settings := sc.settingsService.GetSection(constant.SectionScheduler)

	return &TAGetSchedulerSettingsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: settings,
		},
	}, nil
}

// TAUpdateSchedulerSettingsInput 更新调度设置
type TAUpdateSchedulerSettingsInput struct {
	Body struct {
		WorkerCount  string `json:"worker_count" description:"工作线程数"`
		QueueSize    string `json:"queue_size" description:"等待队列容量"`
		RateInterval string `json:"rate_interval" description:"限频间隔"`
	}
}

// TAUpdateSchedulerSettingsOutput 更新调度设置
type TAUpdateSchedulerSettingsOutput struct {
	Body utils.HumaResponse[any]
}

// TAUpdateSchedulerSettings 更新调度设置
func (sc *SettingsController) TAUpdateSchedulerSettings(ctx context.Context, input *TAUpdateSchedulerSettingsInput) (*TAUpdateSchedulerSettingsOutput, error) {
	req := input.Body

	var workerCount int
	if _, err := fmt.Sscanf(req.WorkerCount, "%d", &workerCount); err != nil || workerCount < 1 || workerCount > 1000 {
		return nil, utils.HumaBadRequest("工作线程数必须在 1 至 1000 之间")
	}

	var queueSize int
	if _, err := fmt.Sscanf(req.QueueSize, "%d", &queueSize); err != nil || queueSize < 1 || queueSize > 50000 {
		return nil, utils.HumaBadRequest("等待队列容量必须在 1 至 50000 之间")
	}

	var rateInterval int
	if _, err := fmt.Sscanf(req.RateInterval, "%d", &rateInterval); err != nil || rateInterval < 1 {
		return nil, utils.HumaBadRequest("限频间隔必须为正整数")
	}

	values := map[string]string{
		constant.KeyWorkerCount:  req.WorkerCount,
		constant.KeyQueueSize:    req.QueueSize,
		constant.KeyRateInterval: req.RateInterval,
	}

	if err := sc.settingsService.SetSection(constant.SectionScheduler, values); err != nil {
		return nil, utils.HumaServerError("保存失败")
	}

	if sc.executorService != nil {
		sc.executorService.Reload()
	}

	return &TAUpdateSchedulerSettingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// TAGetPathsOutput 获取系统路径信息
type TAGetPathsOutput struct {
	Body utils.HumaResponse[struct {
		ScriptsDir string `json:"scripts_dir"`
	}]
}

// TAGetPaths 获取系统路径信息
func (sc *SettingsController) TAGetPaths(ctx context.Context, input *struct{}) (*TAGetPathsOutput, error) {
	absScriptsDir, _ := filepath.Abs(constant.ScriptsWorkDir)

	return &TAGetPathsOutput{
		Body: utils.HumaResponse[struct {
			ScriptsDir string `json:"scripts_dir"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				ScriptsDir string `json:"scripts_dir"`
			}{ScriptsDir: absScriptsDir},
		},
	}, nil
}

// TAGetAboutOutput 获取关于信息
type TAGetAboutOutput struct {
	Body utils.HumaResponse[struct {
		Version       string `json:"version"`
		RemoteVersion string `json:"remote_version"`
		BuildTime     string `json:"build_time"`
		MemUsage      string `json:"mem_usage"`
		Goroutines    int    `json:"goroutines"`
		Uptime        string `json:"uptime"`
		TaskCount     int64  `json:"task_count"`
		LogCount      int64  `json:"log_count"`
		EnvCount      int64  `json:"env_count"`
	}]
}

// TAGetAbout 获取关于信息
func (sc *SettingsController) TAGetAbout(ctx context.Context, input *struct{}) (*TAGetAboutOutput, error) {
	var taskCount, logCount, envCount int64
	database.DB.Model(&models.Task{}).Count(&taskCount)
	database.DB.Model(&models.TaskLog{}).Count(&logCount)
	database.DB.Model(&models.EnvironmentVariable{}).Count(&envCount)

	memUsage := "N/A"
	if p, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if memInfo, err := p.MemoryInfo(); err == nil {
			memUsage = formatBytes(memInfo.RSS)
		}
	}

	uptime := formatDuration(time.Since(constant.StartTime))

	remoteVersion := ""
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/engigu/baihu-panel/releases/latest", nil)
	if err == nil {
		req.Header.Set("User-Agent", "baihu-panel")
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			var release struct {
				TagName string `json:"tag_name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&release); err == nil {
				remoteVersion = release.TagName
			}
		}
	}

	return &TAGetAboutOutput{
		Body: utils.HumaResponse[struct {
			Version       string `json:"version"`
			RemoteVersion string `json:"remote_version"`
			BuildTime     string `json:"build_time"`
			MemUsage      string `json:"mem_usage"`
			Goroutines    int    `json:"goroutines"`
			Uptime        string `json:"uptime"`
			TaskCount     int64  `json:"task_count"`
			LogCount      int64  `json:"log_count"`
			EnvCount      int64  `json:"env_count"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Version       string `json:"version"`
				RemoteVersion string `json:"remote_version"`
				BuildTime     string `json:"build_time"`
				MemUsage      string `json:"mem_usage"`
				Goroutines    int    `json:"goroutines"`
				Uptime        string `json:"uptime"`
				TaskCount     int64  `json:"task_count"`
				LogCount      int64  `json:"log_count"`
				EnvCount      int64  `json:"env_count"`
			}{
				Version:       constant.Version,
				RemoteVersion: remoteVersion,
				BuildTime:     constant.BuildTime,
				MemUsage:      memUsage,
				Goroutines:    runtime.NumGoroutine(),
				Uptime:        uptime,
				TaskCount:     taskCount,
				LogCount:      logCount,
				EnvCount:      envCount,
			},
		},
	}, nil
}

// TAGetChangelogOutput 获取更新日志
type TAGetChangelogOutput struct {
	Body utils.HumaResponse[string]
}

// TAGetChangelog 获取更新日志
func (sc *SettingsController) TAGetChangelog(ctx context.Context, input *struct{}) (*TAGetChangelogOutput, error) {
	content, err := os.ReadFile("docs/guide/changelog.md")
	data := "暂无更新日志"
	if err == nil {
		data = string(content)
	}

	return &TAGetChangelogOutput{
		Body: utils.HumaResponse[string]{
			Code: 200,
			Msg:  "success",
			Data: data,
		},
	}, nil
}

// TAGetLoginLogsInput 获取登录日志
type TAGetLoginLogsInput struct {
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
	Username string `query:"username" description:"用户名"`
}

// TAGetLoginLogsOutput 获取登录日志
type TAGetLoginLogsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.LoginLogVO]]
}

// TAGetLoginLogs 获取登录日志
func (sc *SettingsController) TAGetLoginLogs(ctx context.Context, input *TAGetLoginLogsInput) (*TAGetLoginLogsOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	logs, total, err := sc.loginLogService.List(page, pageSize, input.Username)
	if err != nil {
		return nil, utils.HumaServerError("获取登录日志失败")
	}

	vos := make([]*vo.LoginLogVO, len(logs))
	for i, log := range logs {
		vos[i] = &vo.LoginLogVO{
			ID:        log.ID,
			Username:  log.Title,
			IP:        log.RefID,
			UserAgent: string(log.Content),
			Status:    log.Status,
			Message:   string(log.ErrorMsg),
			CreatedAt: log.CreatedAt,
		}
	}

	return &TAGetLoginLogsOutput{
		Body: utils.HumaResponse[utils.HumaPagination[[]*vo.LoginLogVO]]{
			Code: 200,
			Msg:  "success",
			Data: utils.HumaPagination[[]*vo.LoginLogVO]{
				Data:     vos,
				Total:    total,
				Page:     page,
				PageSize: pageSize,
			},
		},
	}, nil
}

// TACreateBackupOutput 创建备份
type TACreateBackupOutput struct {
	Body utils.HumaResponse[any]
}

// TACreateBackup 创建备份
func (sc *SettingsController) TACreateBackup(ctx context.Context, input *struct{}) (*TACreateBackupOutput, error) {
	_, err := sc.backupService.CreateBackup()
	if err != nil {
		return nil, utils.HumaServerError("创建备份失败: " + err.Error())
	}

	return &TACreateBackupOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "备份创建成功",
		},
	}, nil
}

// TAGetBackupStatusOutput 获取备份状态
type TAGetBackupStatusOutput struct {
	Body utils.HumaResponse[struct {
		HasBackup  bool   `json:"has_backup"`
		BackupTime string `json:"backup_time"`
	}]
}

// TAGetBackupStatus 获取备份状态
func (sc *SettingsController) TAGetBackupStatus(ctx context.Context, input *struct{}) (*TAGetBackupStatusOutput, error) {
	filePath := sc.backupService.GetBackupFile()
	var backupTime string
	if filePath != "" {
		if info, err := os.Stat(filePath); err == nil {
			backupTime = info.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	return &TAGetBackupStatusOutput{
		Body: utils.HumaResponse[struct {
			HasBackup  bool   `json:"has_backup"`
			BackupTime string `json:"backup_time"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				HasBackup  bool   `json:"has_backup"`
				BackupTime string `json:"backup_time"`
			}{
				HasBackup:  filePath != "",
				BackupTime: backupTime,
			},
		},
	}, nil
}

// TAGetSectionSettingsInput 获取指定 section 的设置
type TAGetSectionSettingsInput struct {
	Section string `path:"section" description:"配置 section"`
}

// TAGetSectionSettingsOutput 获取指定 section 的设置
type TAGetSectionSettingsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// TAGetSectionSettings 获取指定 section 的所有设置
func (sc *SettingsController) TAGetSectionSettings(ctx context.Context, input *TAGetSectionSettingsInput) (*TAGetSectionSettingsOutput, error) {
	if input.Section == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	settings := sc.settingsService.GetSection(input.Section)

	return &TAGetSectionSettingsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: settings,
		},
	}, nil
}

// TAUpdateSectionSettingsInput 批量更新 section 的设置
type TAUpdateSectionSettingsInput struct {
	Section string `path:"section" description:"配置 section"`
	Body    map[string]string
}

// TAUpdateSectionSettingsOutput 批量更新 section 的设置
type TAUpdateSectionSettingsOutput struct {
	Body utils.HumaResponse[any]
}

// TAUpdateSectionSettings 批量更新指定 section 的设置
func (sc *SettingsController) TAUpdateSectionSettings(ctx context.Context, input *TAUpdateSectionSettingsInput) (*TAUpdateSectionSettingsOutput, error) {
	if input.Section == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	if err := sc.settingsService.SetSection(input.Section, input.Body); err != nil {
		return nil, utils.HumaServerError("更新失败")
	}

	// 当互联配置发生改变时，通知 tunnel 模块立刻应用新角色
	if input.Section == constant.SectionInterconnect {
		if role, ok := input.Body[constant.KeyInterconnectRole]; ok {
			tunnel.ApplyRole(role)
		}
	}

	return &TAUpdateSectionSettingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// TAGetSettingInput 获取单个设置值
type TAGetSettingInput struct {
	Section string `path:"section" description:"配置 section"`
	Key     string `path:"key" description:"配置 key"`
}

// TAGetSettingOutput 获取单个设置值
type TAGetSettingOutput struct {
	Body utils.HumaResponse[string]
}

// TAGetSetting 获取单个设置值
func (sc *SettingsController) TAGetSetting(ctx context.Context, input *TAGetSettingInput) (*TAGetSettingOutput, error) {
	if input.Section == "" || input.Key == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	value := sc.settingsService.Get(input.Section, input.Key)

	return &TAGetSettingOutput{
		Body: utils.HumaResponse[string]{
			Code: 200,
			Msg:  "success",
			Data: value,
		},
	}, nil
}

// TAGenerateSettingTokenInput 为指定设置生成 token
type TAGenerateSettingTokenInput struct {
	Section string `path:"section" description:"配置 section"`
	Key     string `path:"key" description:"配置 key"`
}

// TAGenerateSettingTokenOutput 为指定设置生成 token
type TAGenerateSettingTokenOutput struct {
	Body utils.HumaResponse[string]
}

// TAGenerateSettingToken 为指定设置生成随机 token
func (sc *SettingsController) TAGenerateSettingToken(ctx context.Context, input *TAGenerateSettingTokenInput) (*TAGenerateSettingTokenOutput, error) {
	if input.Section == "" || input.Key == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	token := strings.ToLower(utils.RandomString(32))

	if err := sc.settingsService.Set(input.Section, input.Key, token); err != nil {
		return nil, utils.HumaServerError("保存失败")
	}

	return &TAGenerateSettingTokenOutput{
		Body: utils.HumaResponse[string]{
			Code: 200,
			Msg:  "success",
			Data: token,
		},
	}, nil
}

// RegisterAPISettingsRoutes 注册 /api/v1 系统设置 Huma 路由
func (sc *SettingsController) RegisterAPISettingsRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/settings/password",
		OperationID: "apiChangePassword",
		Summary:     "修改密码及账号信息",
		Description: "修改当前用户的密码及账号信息",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAChangePassword)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/site",
		OperationID: "apiGetSiteSettings",
		Summary:     "获取站点设置",
		Description: "获取站点设置",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetSiteSettings)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/settings/site",
		OperationID: "apiUpdateSiteSettings",
		Summary:     "更新站点设置",
		Description: "更新站点设置",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAUpdateSiteSettings)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/settings/site/openapi-token/generate",
		OperationID: "apiGenerateOpenapiToken",
		Summary:     "生成 OpenAPI Token",
		Description: "随机生成一个 OpenAPI Token",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGenerateOpenapiToken)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/paths",
		OperationID: "apiGetPaths",
		Summary:     "获取系统路径信息",
		Description: "获取系统脚本目录等路径信息",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetPaths)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/scheduler",
		OperationID: "apiGetSchedulerSettings",
		Summary:     "获取调度设置",
		Description: "获取任务调度设置",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetSchedulerSettings)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/settings/scheduler",
		OperationID: "apiUpdateSchedulerSettings",
		Summary:     "更新调度设置",
		Description: "更新任务调度设置",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAUpdateSchedulerSettings)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/about",
		OperationID: "apiGetAbout",
		Summary:     "获取关于信息",
		Description: "获取系统版本、运行时间等信息",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetAbout)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/changelog",
		OperationID: "apiGetChangelog",
		Summary:     "获取更新日志",
		Description: "获取系统更新日志",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetChangelog)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/loginlogs",
		OperationID: "apiGetLoginLogs",
		Summary:     "获取登录日志",
		Description: "分页获取登录日志",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetLoginLogs)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/settings/backup",
		OperationID: "apiCreateBackup",
		Summary:     "创建备份",
		Description: "创建系统备份",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TACreateBackup)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/backup/status",
		OperationID: "apiGetBackupStatus",
		Summary:     "获取备份状态",
		Description: "获取备份文件状态",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetBackupStatus)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/{section}",
		OperationID: "apiGetSectionSettings",
		Summary:     "获取指定 section 的设置",
		Description: "获取指定配置 section 的所有设置",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetSectionSettings)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/settings/{section}",
		OperationID: "apiUpdateSectionSettings",
		Summary:     "批量更新 section 的设置",
		Description: "批量更新指定配置 section 的设置",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAUpdateSectionSettings)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/settings/{section}/{key}",
		OperationID: "apiGetSetting",
		Summary:     "获取单个设置值",
		Description: "获取指定 section 和 key 的设置值",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGetSetting)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/settings/{section}/{key}/generate",
		OperationID: "apiGenerateSettingToken",
		Summary:     "为指定设置生成 token",
		Description: "为指定 section 和 key 生成随机 token",
		Tags:        []string{"系统设置"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, sc.TAGenerateSettingToken)
}

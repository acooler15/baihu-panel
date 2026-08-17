package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/tunnel"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v3/process"
)

type SettingsController struct {
	userService     *services.UserService
	settingsService *services.SettingsService
	loginLogService *services.LoginLogService
	backupService   *services.BackupService
	executorService *tasks.ExecutorService
}

func NewSettingsController(userService *services.UserService, loginLogService *services.LoginLogService, executorService *tasks.ExecutorService) *SettingsController {
	return &SettingsController{
		userService:     userService,
		settingsService: services.NewSettingsService(),
		loginLogService: loginLogService,
		backupService:   services.NewBackupService(),
		executorService: executorService,
	}
}

// formatBytes 格式化字节数
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟%d秒", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟%d秒", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d分钟%d秒", minutes, seconds)
	}
	return fmt.Sprintf("%d秒", seconds)
}

// ===========================================================================
// 系统设置业务方法（Huma）
// ===========================================================================

// ChangePasswordInput 修改密码
type ChangePasswordInput struct {
	Body struct {
		OldUsername string `json:"old_username" description:"原账号"`
		Username    string `json:"username" description:"新账号"`
		OldPassword string `json:"old_password" description:"原密码"`
		NewPassword string `json:"new_password" description:"新密码"`
	}
}

// ChangePasswordOutput 修改密码
type ChangePasswordOutput struct {
	Body utils.HumaResponse[any]
}

// ChangePassword 修改密码及账号信息
func (sc *SettingsController) ChangePassword(ctx context.Context, input *ChangePasswordInput) (*ChangePasswordOutput, error) {
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
		return &ChangePasswordOutput{
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

	return &ChangePasswordOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  msg,
		},
	}, nil
}

// GetSiteSettingsOutput 获取站点设置
type GetSiteSettingsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// GetSiteSettings 获取站点设置
func (sc *SettingsController) GetSiteSettings(ctx context.Context, input *struct{}) (*GetSiteSettingsOutput, error) {
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

	return &GetSiteSettingsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: settings,
		},
	}, nil
}

// UpdateSiteSettingsInput 更新站点设置
type UpdateSiteSettingsInput struct {
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

// UpdateSiteSettingsOutput 更新站点设置
type UpdateSiteSettingsOutput struct {
	Body utils.HumaResponse[any]
}

// UpdateSiteSettings 更新站点设置
func (sc *SettingsController) UpdateSiteSettings(ctx context.Context, input *UpdateSiteSettingsInput) (*UpdateSiteSettingsOutput, error) {
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

	return &UpdateSiteSettingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// GenerateOpenapiTokenOutput 生成 OpenAPI Token
type GenerateOpenapiTokenOutput struct {
	Body utils.HumaResponse[struct {
		Token string `json:"token"`
	}]
}

// GenerateOpenapiToken 生成 OpenAPI Token
func (sc *SettingsController) GenerateOpenapiToken(ctx context.Context, input *struct{}) (*GenerateOpenapiTokenOutput, error) {
	return &GenerateOpenapiTokenOutput{
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

// GetSchedulerSettingsOutput 获取调度设置
type GetSchedulerSettingsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// GetSchedulerSettings 获取调度设置
func (sc *SettingsController) GetSchedulerSettings(ctx context.Context, input *struct{}) (*GetSchedulerSettingsOutput, error) {
	settings := sc.settingsService.GetSection(constant.SectionScheduler)

	return &GetSchedulerSettingsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: settings,
		},
	}, nil
}

// UpdateSchedulerSettingsInput 更新调度设置
type UpdateSchedulerSettingsInput struct {
	Body struct {
		WorkerCount  string `json:"worker_count" description:"工作线程数"`
		QueueSize    string `json:"queue_size" description:"等待队列容量"`
		RateInterval string `json:"rate_interval" description:"限频间隔"`
	}
}

// UpdateSchedulerSettingsOutput 更新调度设置
type UpdateSchedulerSettingsOutput struct {
	Body utils.HumaResponse[any]
}

// UpdateSchedulerSettings 更新调度设置
func (sc *SettingsController) UpdateSchedulerSettings(ctx context.Context, input *UpdateSchedulerSettingsInput) (*UpdateSchedulerSettingsOutput, error) {
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

	return &UpdateSchedulerSettingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// GetPathsOutput 获取系统路径信息
type GetPathsOutput struct {
	Body utils.HumaResponse[struct {
		ScriptsDir string `json:"scripts_dir"`
	}]
}

// GetPaths 获取系统路径信息
func (sc *SettingsController) GetPaths(ctx context.Context, input *struct{}) (*GetPathsOutput, error) {
	absScriptsDir, _ := filepath.Abs(constant.ScriptsWorkDir)

	return &GetPathsOutput{
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

// GetAboutOutput 获取关于信息
type GetAboutOutput struct {
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

// GetAbout 获取关于信息
func (sc *SettingsController) GetAbout(ctx context.Context, input *struct{}) (*GetAboutOutput, error) {
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

	return &GetAboutOutput{
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

// GetChangelogOutput 获取更新日志
type GetChangelogOutput struct {
	Body utils.HumaResponse[string]
}

// GetChangelog 获取更新日志
func (sc *SettingsController) GetChangelog(ctx context.Context, input *struct{}) (*GetChangelogOutput, error) {
	content, err := os.ReadFile("docs/guide/changelog.md")
	data := "暂无更新日志"
	if err == nil {
		data = string(content)
	}

	return &GetChangelogOutput{
		Body: utils.HumaResponse[string]{
			Code: 200,
			Msg:  "success",
			Data: data,
		},
	}, nil
}

// GetLoginLogsInput 获取登录日志
type GetLoginLogsInput struct {
	Page     int    `query:"page" default:"1" description:"页码"`
	PageSize int    `query:"page_size" default:"10" description:"每页数量"`
	Username string `query:"username" description:"用户名"`
}

// GetLoginLogsOutput 获取登录日志
type GetLoginLogsOutput struct {
	Body utils.HumaResponse[utils.HumaPagination[[]*vo.LoginLogVO]]
}

// GetLoginLogs 获取登录日志
func (sc *SettingsController) GetLoginLogs(ctx context.Context, input *GetLoginLogsInput) (*GetLoginLogsOutput, error) {
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

	return &GetLoginLogsOutput{
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

// CreateBackupOutput 创建备份
type CreateBackupOutput struct {
	Body utils.HumaResponse[any]
}

// CreateBackup 创建备份
func (sc *SettingsController) CreateBackup(ctx context.Context, input *struct{}) (*CreateBackupOutput, error) {
	_, err := sc.backupService.CreateBackup()
	if err != nil {
		return nil, utils.HumaServerError("创建备份失败: " + err.Error())
	}

	return &CreateBackupOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "备份创建成功",
		},
	}, nil
}

// GetBackupStatusOutput 获取备份状态
type GetBackupStatusOutput struct {
	Body utils.HumaResponse[struct {
		HasBackup  bool   `json:"has_backup"`
		BackupTime string `json:"backup_time"`
	}]
}

// GetBackupStatus 获取备份状态
func (sc *SettingsController) GetBackupStatus(ctx context.Context, input *struct{}) (*GetBackupStatusOutput, error) {
	filePath := sc.backupService.GetBackupFile()
	var backupTime string
	if filePath != "" {
		if info, err := os.Stat(filePath); err == nil {
			backupTime = info.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	return &GetBackupStatusOutput{
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

// GetSectionSettingsInput 获取指定 section 的设置
type GetSectionSettingsInput struct {
	Section string `path:"section" description:"配置 section"`
}

// GetSectionSettingsOutput 获取指定 section 的设置
type GetSectionSettingsOutput struct {
	Body utils.HumaResponse[map[string]string]
}

// GetSectionSettings 获取指定 section 的所有设置
func (sc *SettingsController) GetSectionSettings(ctx context.Context, input *GetSectionSettingsInput) (*GetSectionSettingsOutput, error) {
	if input.Section == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	settings := sc.settingsService.GetSection(input.Section)

	return &GetSectionSettingsOutput{
		Body: utils.HumaResponse[map[string]string]{
			Code: 200,
			Msg:  "success",
			Data: settings,
		},
	}, nil
}

// UpdateSectionSettingsInput 批量更新 section 的设置
type UpdateSectionSettingsInput struct {
	Section string `path:"section" description:"配置 section"`
	Body    map[string]string
}

// UpdateSectionSettingsOutput 批量更新 section 的设置
type UpdateSectionSettingsOutput struct {
	Body utils.HumaResponse[any]
}

// UpdateSectionSettings 批量更新指定 section 的设置
func (sc *SettingsController) UpdateSectionSettings(ctx context.Context, input *UpdateSectionSettingsInput) (*UpdateSectionSettingsOutput, error) {
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

	return &UpdateSectionSettingsOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// GetSettingInput 获取单个设置值
type GetSettingInput struct {
	Section string `path:"section" description:"配置 section"`
	Key     string `path:"key" description:"配置 key"`
}

// GetSettingOutput 获取单个设置值
type GetSettingOutput struct {
	Body utils.HumaResponse[string]
}

// GetSetting 获取单个设置值
func (sc *SettingsController) GetSetting(ctx context.Context, input *GetSettingInput) (*GetSettingOutput, error) {
	if input.Section == "" || input.Key == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	value := sc.settingsService.Get(input.Section, input.Key)

	return &GetSettingOutput{
		Body: utils.HumaResponse[string]{
			Code: 200,
			Msg:  "success",
			Data: value,
		},
	}, nil
}

// GenerateSettingTokenInput 为指定设置生成 token
type GenerateSettingTokenInput struct {
	Section string `path:"section" description:"配置 section"`
	Key     string `path:"key" description:"配置 key"`
}

// GenerateSettingTokenOutput 为指定设置生成 token
type GenerateSettingTokenOutput struct {
	Body utils.HumaResponse[string]
}

// GenerateSettingToken 为指定设置生成随机 token
func (sc *SettingsController) GenerateSettingToken(ctx context.Context, input *GenerateSettingTokenInput) (*GenerateSettingTokenOutput, error) {
	if input.Section == "" || input.Key == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	token := strings.ToLower(utils.RandomString(32))

	if err := sc.settingsService.Set(input.Section, input.Key, token); err != nil {
		return nil, utils.HumaServerError("保存失败")
	}

	return &GenerateSettingTokenOutput{
		Body: utils.HumaResponse[string]{
			Code: 200,
			Msg:  "success",
			Data: token,
		},
	}, nil
}

// ===========================================================================
// 备份下载 / 恢复（Huma 流式实现）
// ===========================================================================

// DownloadBackupHuma 下载系统备份（流式输出）
func (sc *SettingsController) DownloadBackupHuma(ctx context.Context, input *struct{}) (*huma.StreamResponse, error) {
	filePath := sc.backupService.GetBackupFile()
	if filePath == "" {
		return nil, utils.HumaNotFound("没有可下载的备份")
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		sc.backupService.ClearBackup()
		return nil, utils.HumaNotFound("备份文件不存在")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, utils.HumaServerError("无法打开备份文件")
	}

	filename := filepath.Base(filePath)
	return &huma.StreamResponse{
		Body: func(hctx huma.Context) {
			defer file.Close()
			hctx.SetHeader("Content-Type", "application/zip")
			hctx.SetHeader("Content-Disposition", `attachment; filename="`+filename+`"`)
			io.Copy(hctx.BodyWriter(), file)
		},
	}, nil
}

// RestoreBackupHumaInput 恢复备份
type RestoreBackupHumaInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" description:"备份文件"`
	}]
}

// RestoreBackupHumaOutput 恢复备份结果
type RestoreBackupHumaOutput struct {
	Body utils.HumaResponse[any]
}

// RestoreBackupHuma 恢复系统备份
func (sc *SettingsController) RestoreBackupHuma(ctx context.Context, input *RestoreBackupHumaInput) (*RestoreBackupHumaOutput, error) {
	data := input.RawBody.Data()
	if !data.File.IsSet {
		return nil, utils.HumaBadRequest("请上传备份文件")
	}

	// 保存上传的文件
	tempPath := filepath.Join(os.TempDir(), filepath.Base(data.File.Filename))
	dst, err := os.Create(tempPath)
	if err != nil {
		return nil, utils.HumaServerError("保存文件失败")
	}
	if _, err := io.Copy(dst, data.File); err != nil {
		dst.Close()
		os.Remove(tempPath)
		return nil, utils.HumaServerError("保存文件失败")
	}
	dst.Close()
	defer os.Remove(tempPath)

	// 恢复备份
	if err := sc.backupService.Restore(tempPath); err != nil {
		return nil, utils.HumaServerError("恢复失败: " + err.Error())
	}

	return &RestoreBackupHumaOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "恢复成功",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPISettingsRoutes 注册 /api/v1 系统设置 Huma 路由
func (sc *SettingsController) RegisterAPISettingsRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"系统设置"}

	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/settings/password", OperationID: "ChangePassword", Summary: "修改密码及账号信息", Description: "修改当前用户的密码及账号信息", Tags: tag, Security: security}, sc.ChangePassword)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/site", OperationID: "GetSiteSettings", Summary: "获取站点设置", Description: "获取站点设置", Tags: tag, Security: security}, sc.GetSiteSettings)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/settings/site", OperationID: "UpdateSiteSettings", Summary: "更新站点设置", Description: "更新站点设置", Tags: tag, Security: security}, sc.UpdateSiteSettings)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/settings/site/openapi-token/generate", OperationID: "GenerateOpenapiToken", Summary: "生成 OpenAPI Token", Description: "随机生成一个 OpenAPI Token", Tags: tag, Security: security}, sc.GenerateOpenapiToken)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/paths", OperationID: "GetPaths", Summary: "获取系统路径信息", Description: "获取系统脚本目录等路径信息", Tags: tag, Security: security}, sc.GetPaths)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/scheduler", OperationID: "GetSchedulerSettings", Summary: "获取调度设置", Description: "获取任务调度设置", Tags: tag, Security: security}, sc.GetSchedulerSettings)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/settings/scheduler", OperationID: "UpdateSchedulerSettings", Summary: "更新调度设置", Description: "更新任务调度设置", Tags: tag, Security: security}, sc.UpdateSchedulerSettings)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/about", OperationID: "GetAbout", Summary: "获取关于信息", Description: "获取系统版本、运行时间等信息", Tags: tag, Security: security}, sc.GetAbout)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/changelog", OperationID: "GetChangelog", Summary: "获取更新日志", Description: "获取系统更新日志", Tags: tag, Security: security}, sc.GetChangelog)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/loginlogs", OperationID: "GetLoginLogs", Summary: "获取登录日志", Description: "分页获取登录日志", Tags: tag, Security: security}, sc.GetLoginLogs)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/settings/backup", OperationID: "CreateBackup", Summary: "创建备份", Description: "创建系统备份", Tags: tag, Security: security}, sc.CreateBackup)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/backup/status", OperationID: "GetBackupStatus", Summary: "获取备份状态", Description: "获取备份文件状态", Tags: tag, Security: security}, sc.GetBackupStatus)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/{section}", OperationID: "GetSectionSettings", Summary: "获取指定 section 的设置", Description: "获取指定配置 section 的所有设置", Tags: tag, Security: security}, sc.GetSectionSettings)
	huma.Register(api, huma.Operation{Method: http.MethodPut, Path: "/settings/{section}", OperationID: "UpdateSectionSettings", Summary: "批量更新 section 的设置", Description: "批量更新指定配置 section 的设置", Tags: tag, Security: security}, sc.UpdateSectionSettings)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/{section}/{key}", OperationID: "GetSetting", Summary: "获取单个设置值", Description: "获取指定 section 和 key 的设置值", Tags: tag, Security: security}, sc.GetSetting)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/settings/{section}/{key}/generate", OperationID: "GenerateSettingToken", Summary: "为指定设置生成 token", Description: "为指定 section 和 key 生成随机 token", Tags: tag, Security: security}, sc.GenerateSettingToken)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/backup/download", OperationID: "DownloadBackup", Summary: "下载系统备份", Description: "下载最近一次生成的系统备份 Zip 文件（流式）。", Tags: tag, Security: security}, sc.DownloadBackupHuma)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/settings/restore", OperationID: "RestoreBackup", Summary: "恢复系统备份", Description: "以 multipart/form-data 上传备份 Zip 文件并执行恢复。", Tags: tag, Security: security}, sc.RestoreBackupHuma)

	// 公开站点设置（无需鉴权，selector 中按路径放行）
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/settings/public", OperationID: "GetPublicSiteSettings", Summary: "获取公开站点设置", Description: "无需认证即可获取站点标题、副标题、图标等公开信息。", Tags: tag}, sc.GetPublicSiteSettingsHuma)
}

// ===========================================================================
// 公开站点设置（Huma，迁移自 Gin 原生 GetPublicSiteSettings）
// ===========================================================================

// GetPublicSiteSettingsOutput 公开站点设置结果
type GetPublicSiteSettingsOutput struct {
	Body utils.HumaResponse[struct {
		Title     string `json:"title,omitempty"`
		Subtitle  string `json:"subtitle,omitempty"`
		Icon      string `json:"icon,omitempty"`
		DemoMode  bool   `json:"demo_mode,omitempty"`
	}]
}

// GetPublicSiteSettingsHuma 获取公开站点设置（无需认证）
func (sc *SettingsController) GetPublicSiteSettingsHuma(ctx context.Context, input *struct{}) (*GetPublicSiteSettingsOutput, error) {
	settings := sc.settingsService.GetSection(constant.SectionSite)

	title := settings[constant.KeyTitle]
	if title == "" {
		title = "白虎面板"
	}
	subtitle := settings[constant.KeySubtitle]
	if subtitle == "" {
		subtitle = "极致轻量、高性能的自动化任务调度平台"
	}
	icon := settings[constant.KeyIcon]
	if icon == "" {
		icon = constant.DefaultIcon
	}

	return &GetPublicSiteSettingsOutput{
		Body: utils.HumaResponse[struct {
			Title     string `json:"title,omitempty"`
			Subtitle  string `json:"subtitle,omitempty"`
			Icon      string `json:"icon,omitempty"`
			DemoMode  bool   `json:"demo_mode,omitempty"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Title     string `json:"title,omitempty"`
				Subtitle  string `json:"subtitle,omitempty"`
				Icon      string `json:"icon,omitempty"`
				DemoMode  bool   `json:"demo_mode,omitempty"`
			}{
				Title:     title,
				Subtitle:  subtitle,
				Icon:      icon,
				DemoMode:  constant.DemoMode,
			},
		},
	}, nil
}

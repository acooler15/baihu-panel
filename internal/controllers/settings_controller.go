package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/tasks"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
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

// GetPublicSiteSettings 获取公开的站点设置（无需认证）
func (sc *SettingsController) GetPublicSiteSettings(c *gin.Context) {
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

	// 只返回公开信息
	c.JSON(http.StatusOK, utils.Response{
		Code: 200,
		Msg:  "success",
		Data: gin.H{
			constant.KeyTitle:    title,
			constant.KeySubtitle: subtitle,
			constant.KeyIcon:     icon,
			"demo_mode":          constant.DemoMode,
		},
	})
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

// DownloadBackup 下载备份文件
func (sc *SettingsController) DownloadBackup(c *gin.Context) {
	filePath := sc.backupService.GetBackupFile()
	if filePath == "" {
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "没有可下载的备份"})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		sc.backupService.ClearBackup()
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "备份文件不存在"})
		return
	}

	// 设置响应头
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	c.Header("Content-Type", "application/zip")
	c.File(filePath)

	// 下载后清除备份记录和文件
	go func() {
		time.Sleep(time.Minute * 5) // 等待下载完成
		sc.backupService.ClearBackup()
	}()
}

// RestoreBackup 恢复备份
func (sc *SettingsController) RestoreBackup(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "请上传备份文件"})
		return
	}

	// 保存上传的文件
	tempPath := filepath.Join(os.TempDir(), file.Filename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "保存文件失败"})
		return
	}
	defer os.Remove(tempPath)

	// 恢复备份
	if err := sc.backupService.Restore(tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "恢复失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "恢复成功"})
}

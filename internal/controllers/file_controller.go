package controllers

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
	"net/http"
)

var (
	extractZip   = utils.ExtractZip
	extractTar   = utils.ExtractTar
	extractTarGz = utils.ExtractTarGz
)

type FileController struct {
	workDir string
}

func NewFileController(workDir string) *FileController {
	os.MkdirAll(workDir, 0755)
	absPath, err := filepath.Abs(workDir)
	if err != nil {
		absPath = workDir
	}
	return &FileController{workDir: absPath}
}

// FileNode 文件树节点
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	ModTime  int64       `json:"modTime"`
	Children []*FileNode `json:"children,omitempty"`
}

// checkPath 校验路径是否在工作目录内且安全。
// 它返回完整的绝对路径以及一个表示路径是否安全的布尔值。
func (fc *FileController) checkPath(path string, allowRoot bool) (string, bool) {
	fullPath := filepath.Join(fc.workDir, filepath.Clean(path))
	rel, err := filepath.Rel(fc.workDir, fullPath)
	if err != nil {
		return "", false
	}

	// 基础的目录穿越检查
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	// 根目录检查
	if !allowRoot && rel == "." {
		return "", false
	}

	return fullPath, true
}

// UploadArchive 处理归档文件的上传和解压
func (fc *FileController) UploadArchive(c *gin.Context) {
	targetDir := c.PostForm("path")

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "请选择文件"})
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".zip" && ext != ".tar" && ext != ".gz" && ext != ".tgz" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "仅支持 zip、tar、gz、tgz 格式"})
		return
	}

	// 确定解压目标目录
	extractDir, safe := fc.checkPath(targetDir, true)
	if !safe {
		c.JSON(http.StatusForbidden, utils.Response{Code: 403, Msg: "访问被拒绝"})
		return
	}
	os.MkdirAll(extractDir, 0755)

	// 保存临时文件
	// 安全修复：使用 filepath.Base 提取纯文件名，防止路径穿越攻击
	tempFile := filepath.Join(os.TempDir(), filepath.Base(file.Filename))
	if err := c.SaveUploadedFile(file, tempFile); err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "保存文件失败"})
		return
	}
	defer os.Remove(tempFile)

	// 解压文件
	var extractErr error
	switch {
	case ext == ".zip":
		extractErr = extractZip(tempFile, extractDir)
	case ext == ".tar":
		extractErr = extractTar(tempFile, extractDir)
	case ext == ".gz" || ext == ".tgz":
		extractErr = extractTarGz(tempFile, extractDir)
	}

	if extractErr != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "解压失败: " + extractErr.Error()})
		return
	}

	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "导入成功"})
}

// UploadFiles 处理多个文件的上传
func (fc *FileController) UploadFiles(c *gin.Context) {
	targetDir := c.PostForm("path")

	// 确定目标目录
	destDir, safe := fc.checkPath(targetDir, true)
	if !safe {
		c.JSON(http.StatusForbidden, utils.Response{Code: 403, Msg: "访问被拒绝"})
		return
	}
	os.MkdirAll(destDir, 0755)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "请选择文件"})
		return
	}

	files := form.File["files"]
	paths := form.Value["paths"] // 相对路径数组，用于保持文件夹结构

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "请选择文件"})
		return
	}

	for i, file := range files {
		// 获取相对路径（如果有）
		// 安全修复：清理文件名
		relPath := filepath.Base(file.Filename)
		if i < len(paths) && paths[i] != "" {
			relPath = paths[i]
		}

		// 构建完整路径
		fullPath, safe := fc.checkPath(filepath.Join(targetDir, relPath), false)
		if !safe {
			continue
		}

		// 确保父目录存在
		os.MkdirAll(filepath.Dir(fullPath), 0755)

		// 保存文件
		if err := c.SaveUploadedFile(file, fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "保存文件失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, utils.Response{Code: 200, Msg: "上传成功"})
}

// DownloadFile 下载单个文件
func (fc *FileController) DownloadFile(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "path参数必填"})
		return
	}

	fullPath, safe := fc.checkPath(filePath, false)
	if !safe {
		c.JSON(http.StatusForbidden, utils.Response{Code: 403, Msg: "访问被拒绝"})
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "文件不存在"})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(fullPath))
	c.Header("Content-Type", "application/octet-stream")
	c.File(fullPath)
}

// DownloadZip 批量下载为 zip
func (fc *FileController) DownloadZip(c *gin.Context) {
	paths := c.QueryArray("path")
	if len(paths) == 0 || c.ContentType() == "application/json" {
		var req struct {
			Paths []string `json:"paths"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && len(paths) == 0 {
			paths = req.Paths
		}
	}

	if len(paths) == 0 {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "path参数必填"})
		return
	}

	validatedAbsPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		fullPath, safe := fc.checkPath(path, false)
		if !safe {
			c.JSON(http.StatusForbidden, utils.Response{Code: 403, Msg: "访问被拒绝"})
			return
		}

		if _, err := os.Stat(fullPath); err != nil {
			c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "文件不存在"})
			return
		}

		validatedAbsPaths = append(validatedAbsPaths, fullPath)
	}

	fileName := "baihu-export-" + time.Now().Format("20060102-150405") + ".zip"
	if len(validatedAbsPaths) == 1 {
		fileName = filepath.Base(validatedAbsPaths[0]) + ".zip"
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/zip")

	if err := utils.CreateZip(c.Writer, validatedAbsPaths); err != nil {
		return
	}
}

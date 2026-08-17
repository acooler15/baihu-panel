package controllers

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"

	"github.com/danielgtaylor/huma/v2"
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

// buildFileTree 构建文件树数据（复用 GetFileTree 的原始逻辑）
func (fc *FileController) buildFileTree(root *FileNode) error {
	return filepath.WalkDir(fc.workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == fc.workDir {
			return nil
		}

		// 过滤 __pycache__ 文件夹
		if d.IsDir() && d.Name() == "__pycache__" {
			return filepath.SkipDir
		}

		relPath, _ := filepath.Rel(fc.workDir, path)
		parts := strings.Split(relPath, string(filepath.Separator))

		info, err := d.Info()
		var modTime int64
		if err == nil {
			modTime = info.ModTime().UnixMilli()
		}

		current := root
		for i, part := range parts {
			found := false
			for _, child := range current.Children {
				if child.Name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				isLast := i == len(parts)-1
				isDir := !isLast || d.IsDir()
				node := &FileNode{
					Name:    part,
					Path:    strings.Join(parts[:i+1], "/"),
					IsDir:   isDir,
					ModTime: modTime,
				}
				if isDir {
					node.Children = []*FileNode{}
				}
				current.Children = append(current.Children, node)
				current = node
			}
		}
		return nil
	})
}

// ===========================================================================
// Gin 原生 handler（由 api_routes.go 保留引用）
// ===========================================================================

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

// ===========================================================================
// 文件管理业务方法（Huma）
// ===========================================================================

// GetFileTreeOutput 获取文件树
type GetFileTreeOutput struct {
	Body utils.HumaResponse[[]*FileNode]
}

// GetFileTree 获取脚本目录的文件树
func (fc *FileController) GetFileTree(ctx context.Context, input *struct{}) (*GetFileTreeOutput, error) {
	root := &FileNode{
		Name:     filepath.Base(fc.workDir),
		Path:     "",
		IsDir:    true,
		Children: []*FileNode{},
	}

	if err := fc.buildFileTree(root); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &GetFileTreeOutput{
		Body: utils.HumaResponse[[]*FileNode]{
			Code: 200,
			Msg:  "success",
			Data: root.Children,
		},
	}, nil
}

// GetFileContentInput 获取文件内容
type GetFileContentInput struct {
	Path string `query:"path" description:"文件路径"`
}

// GetFileContentOutput 获取文件内容
type GetFileContentOutput struct {
	Body utils.HumaResponse[struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}]
}

// GetFileContent 获取文件内容
func (fc *FileController) GetFileContent(ctx context.Context, input *GetFileContentInput) (*GetFileContentOutput, error) {
	filePath := input.Path
	if filePath == "" {
		return nil, utils.HumaBadRequest("path参数必填")
	}

	fullPath, safe := fc.checkPath(filePath, false)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, utils.HumaNotFound("文件不存在")
	}

	return &GetFileContentOutput{
		Body: utils.HumaResponse[struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}{
				Path:    filePath,
				Content: string(content),
			},
		},
	}, nil
}

// SaveFileContentInput 保存文件内容
type SaveFileContentInput struct {
	Body struct {
		Path    string `json:"path" description:"文件路径"`
		Content string `json:"content" description:"文件内容"`
	}
}

// SaveFileContentOutput 保存文件内容
type SaveFileContentOutput struct {
	Body utils.HumaResponse[any]
}

// SaveFileContent 保存文件内容
func (fc *FileController) SaveFileContent(ctx context.Context, input *SaveFileContentInput) (*SaveFileContentOutput, error) {
	req := input.Body
	if req.Path == "" {
		return nil, utils.HumaBadRequest("path参数必填")
	}

	fullPath, safe := fc.checkPath(req.Path, false)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	os.MkdirAll(filepath.Dir(fullPath), 0755)

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &SaveFileContentOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// CreateFileInput 创建文件/目录
type CreateFileInput struct {
	Body struct {
		Path  string `json:"path" description:"路径"`
		IsDir bool   `json:"isDir" description:"是否为目录"`
	}
}

// CreateFileOutput 创建文件/目录
type CreateFileOutput struct {
	Body utils.HumaResponse[any]
}

// CreateFile 创建文件或目录
func (fc *FileController) CreateFile(ctx context.Context, input *CreateFileInput) (*CreateFileOutput, error) {
	req := input.Body
	if req.Path == "" {
		return nil, utils.HumaBadRequest("path参数必填")
	}

	fullPath, safe := fc.checkPath(req.Path, false)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	if req.IsDir {
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return nil, utils.HumaServerError(err.Error())
		}
	} else {
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(""), 0644); err != nil {
			return nil, utils.HumaServerError(err.Error())
		}
	}

	return &CreateFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "创建成功",
		},
	}, nil
}

// DeleteFileInput 删除文件/目录
type DeleteFileInput struct {
	Body struct {
		Path string `json:"path" description:"路径"`
	}
}

// DeleteFileOutput 删除文件/目录
type DeleteFileOutput struct {
	Body utils.HumaResponse[any]
}

// DeleteFile 删除文件或目录
func (fc *FileController) DeleteFile(ctx context.Context, input *DeleteFileInput) (*DeleteFileOutput, error) {
	if input.Body.Path == "" {
		return nil, utils.HumaBadRequest("path参数必填")
	}

	fullPath, safe := fc.checkPath(input.Body.Path, false)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	if err := os.RemoveAll(fullPath); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &DeleteFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// MoveFileInput 移动文件
type MoveFileInput struct {
	Body struct {
		OldPath string `json:"oldPath" description:"源路径"`
		NewPath string `json:"newPath" description:"目标路径"`
	}
}

// MoveFileOutput 移动文件
type MoveFileOutput struct {
	Body utils.HumaResponse[any]
}

// MoveFile 移动文件
func (fc *FileController) MoveFile(ctx context.Context, input *MoveFileInput) (*MoveFileOutput, error) {
	req := input.Body
	if req.OldPath == "" || req.NewPath == "" {
		return nil, utils.HumaBadRequest("oldPath 和 newPath 必填")
	}

	oldFull, oldSafe := fc.checkPath(req.OldPath, false)
	newFull, newSafe := fc.checkPath(req.NewPath, false)

	if !oldSafe || !newSafe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	if oldFull == newFull {
		return &MoveFileOutput{
			Body: utils.HumaResponse[any]{
				Code: 200,
				Msg:  "success",
			},
		}, nil
	}

	if _, err := os.Stat(newFull); err == nil {
		return nil, utils.HumaBadRequest("目标已存在")
	}

	os.MkdirAll(filepath.Dir(newFull), 0755)

	if err := os.Rename(oldFull, newFull); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &MoveFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// CopyFileInput 复制文件
type CopyFileInput struct {
	Body struct {
		SourcePath string `json:"sourcePath" description:"源路径"`
		TargetPath string `json:"targetPath" description:"目标路径"`
	}
}

// CopyFileOutput 复制文件
type CopyFileOutput struct {
	Body utils.HumaResponse[any]
}

// CopyFile 复制文件
func (fc *FileController) CopyFile(ctx context.Context, input *CopyFileInput) (*CopyFileOutput, error) {
	req := input.Body
	if req.SourcePath == "" || req.TargetPath == "" {
		return nil, utils.HumaBadRequest("sourcePath 和 targetPath 必填")
	}

	sourceFull, sourceSafe := fc.checkPath(req.SourcePath, false)
	targetFull, targetSafe := fc.checkPath(req.TargetPath, false)

	if !sourceSafe || !targetSafe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	if sourceFull == targetFull {
		return &CopyFileOutput{
			Body: utils.HumaResponse[any]{
				Code: 200,
				Msg:  "success",
			},
		}, nil
	}

	content, err := os.ReadFile(sourceFull)
	if err != nil {
		return nil, utils.HumaNotFound("源文件不存在或无法读取")
	}

	os.MkdirAll(filepath.Dir(targetFull), 0755)

	if _, err := os.Stat(targetFull); err == nil {
		return nil, utils.HumaBadRequest("目标已存在")
	}

	if err := os.WriteFile(targetFull, content, 0644); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &CopyFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// RenameFileInput 重命名文件
type RenameFileInput struct {
	Body struct {
		OldPath string `json:"oldPath" description:"旧路径"`
		NewPath string `json:"newPath" description:"新路径"`
	}
}

// RenameFileOutput 重命名文件
type RenameFileOutput struct {
	Body utils.HumaResponse[any]
}

// RenameFile 重命名文件
func (fc *FileController) RenameFile(ctx context.Context, input *RenameFileInput) (*RenameFileOutput, error) {
	req := input.Body
	if req.OldPath == "" || req.NewPath == "" {
		return nil, utils.HumaBadRequest("oldPath 和 newPath 必填")
	}

	// 校验：重命名禁止跨目录
	if filepath.Dir(filepath.Clean(req.OldPath)) != filepath.Dir(filepath.Clean(req.NewPath)) {
		return nil, utils.HumaBadRequest("禁止跨目录重命名")
	}

	oldFull, oldSafe := fc.checkPath(req.OldPath, false)
	newFull, newSafe := fc.checkPath(req.NewPath, false)

	if !oldSafe || !newSafe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	if oldFull == newFull {
		return &RenameFileOutput{
			Body: utils.HumaResponse[any]{
				Code: 200,
				Msg:  "success",
			},
		}, nil
	}

	if _, err := os.Stat(newFull); err == nil {
		return nil, utils.HumaBadRequest("文件已存在")
	}

	if err := os.Rename(oldFull, newFull); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &RenameFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// ===========================================================================
// 文件下载 / 上传（Huma 流式实现）
// ===========================================================================

// DownloadFileHumaInput 下载单个文件
type DownloadFileHumaInput struct {
	Path string `query:"path" description:"文件路径"`
}

// DownloadFileHuma 下载单个文件（流式输出）
func (fc *FileController) DownloadFileHuma(ctx context.Context, input *DownloadFileHumaInput) (*huma.StreamResponse, error) {
	if input.Path == "" {
		return nil, utils.HumaBadRequest("path参数必填")
	}

	fullPath, safe := fc.checkPath(input.Path, false)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return nil, utils.HumaNotFound("文件不存在")
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, utils.HumaServerError("无法打开文件")
	}

	filename := filepath.Base(fullPath)
	return &huma.StreamResponse{
		Body: func(hctx huma.Context) {
			defer file.Close()
			hctx.SetHeader("Content-Description", "File Transfer")
			hctx.SetHeader("Content-Transfer-Encoding", "binary")
			hctx.SetHeader("Content-Type", "application/octet-stream")
			hctx.SetHeader("Content-Disposition", `attachment; filename="`+filename+`"`)
			io.Copy(hctx.BodyWriter(), file)
		},
	}, nil
}

// DownloadZipHumaInput 批量下载为 zip
type DownloadZipHumaInput struct {
	Path  []string `query:"path" description:"文件/目录路径（可重复）"`
	Paths []string `json:"paths" description:"文件/目录路径数组（JSON 请求体，可选）"`
}

// DownloadZipHuma 批量下载为 Zip 流
func (fc *FileController) DownloadZipHuma(ctx context.Context, input *DownloadZipHumaInput) (*huma.StreamResponse, error) {
	paths := input.Path
	if len(paths) == 0 {
		paths = input.Paths
	}

	if len(paths) == 0 {
		return nil, utils.HumaBadRequest("path参数必填")
	}

	validatedAbsPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		fullPath, safe := fc.checkPath(path, false)
		if !safe {
			return nil, utils.HumaForbidden("访问被拒绝")
		}
		if _, err := os.Stat(fullPath); err != nil {
			return nil, utils.HumaNotFound("文件不存在")
		}
		validatedAbsPaths = append(validatedAbsPaths, fullPath)
	}

	fileName := "baihu-export-" + time.Now().Format("20060102-150405") + ".zip"
	if len(validatedAbsPaths) == 1 {
		fileName = filepath.Base(validatedAbsPaths[0]) + ".zip"
	}

	return &huma.StreamResponse{
		Body: func(hctx huma.Context) {
			hctx.SetHeader("Content-Description", "File Transfer")
			hctx.SetHeader("Content-Transfer-Encoding", "binary")
			hctx.SetHeader("Content-Type", "application/zip")
			hctx.SetHeader("Content-Disposition", `attachment; filename="`+fileName+`"`)
			utils.CreateZip(hctx.BodyWriter(), validatedAbsPaths)
		},
	}, nil
}

// UploadArchiveHumaInput 上传并解压归档
type UploadArchiveHumaInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" description:"归档文件"`
		Path string        `form:"path" description:"解压目标目录"`
	}]
}

// UploadArchiveHumaOutput 上传归档结果
type UploadArchiveHumaOutput struct {
	Body utils.HumaResponse[any]
}

// UploadArchiveHuma 上传并解压归档
func (fc *FileController) UploadArchiveHuma(ctx context.Context, input *UploadArchiveHumaInput) (*UploadArchiveHumaOutput, error) {
	data := input.RawBody.Data()
	if !data.File.IsSet {
		return nil, utils.HumaBadRequest("请选择文件")
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(data.File.Filename))
	if ext != ".zip" && ext != ".tar" && ext != ".gz" && ext != ".tgz" {
		return nil, utils.HumaBadRequest("仅支持 zip、tar、gz、tgz 格式")
	}

	// 确定解压目标目录
	extractDir, safe := fc.checkPath(data.Path, true)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}
	os.MkdirAll(extractDir, 0755)

	// 保存临时文件（使用纯文件名防止路径穿越）
	tempFile := filepath.Join(os.TempDir(), filepath.Base(data.File.Filename))
	dst, err := os.Create(tempFile)
	if err != nil {
		return nil, utils.HumaServerError("保存文件失败")
	}
	if _, err := io.Copy(dst, data.File); err != nil {
		dst.Close()
		os.Remove(tempFile)
		return nil, utils.HumaServerError("保存文件失败")
	}
	dst.Close()
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
		return nil, utils.HumaServerError("解压失败: " + extractErr.Error())
	}

	return &UploadArchiveHumaOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "导入成功",
		},
	}, nil
}

// UploadFilesHumaInput 上传多个文件
type UploadFilesHumaInput struct {
	RawBody huma.MultipartFormFiles[struct {
		Files []huma.FormFile `form:"files" description:"文件列表"`
		Path  string          `form:"path" description:"目标目录"`
		Paths []string        `form:"paths" description:"相对路径数组"`
	}]
}

// UploadFilesHumaOutput 上传多个文件结果
type UploadFilesHumaOutput struct {
	Body utils.HumaResponse[any]
}

// UploadFilesHuma 上传多个文件
func (fc *FileController) UploadFilesHuma(ctx context.Context, input *UploadFilesHumaInput) (*UploadFilesHumaOutput, error) {
	data := input.RawBody.Data()
	targetDir := data.Path
	if len(data.Files) == 0 {
		return nil, utils.HumaBadRequest("请选择文件")
	}

	// 确定目标目录
	destDir, safe := fc.checkPath(targetDir, true)
	if !safe {
		return nil, utils.HumaForbidden("访问被拒绝")
	}
	os.MkdirAll(destDir, 0755)

	for i, f := range data.Files {
		// 获取相对路径（如果有）
		relPath := filepath.Base(f.Filename)
		if i < len(data.Paths) && data.Paths[i] != "" {
			relPath = data.Paths[i]
		}

		// 构建完整路径
		fullPath, fileSafe := fc.checkPath(filepath.Join(targetDir, relPath), false)
		if !fileSafe {
			continue
		}

		// 确保父目录存在
		os.MkdirAll(filepath.Dir(fullPath), 0755)

		// 保存文件
		out, err := os.Create(fullPath)
		if err != nil {
			return nil, utils.HumaServerError("保存文件失败: " + err.Error())
		}
		if _, err := io.Copy(out, f); err != nil {
			out.Close()
			return nil, utils.HumaServerError("保存文件失败: " + err.Error())
		}
		out.Close()
	}

	return &UploadFilesHumaOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "上传成功",
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIFileRoutes 注册 /api/v1 文件管理 Huma 路由
func (fc *FileController) RegisterAPIFileRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"文件管理"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/files/tree", OperationID: "GetFileTree", Summary: "获取文件树", Description: "获取脚本目录的文件树", Tags: tag, Security: security}, fc.GetFileTree)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/files/content", OperationID: "GetFileContent", Summary: "获取文件内容", Description: "获取指定文件的内容", Tags: tag, Security: security}, fc.GetFileContent)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/content", OperationID: "SaveFileContent", Summary: "保存文件内容", Description: "保存指定文件的内容", Tags: tag, Security: security}, fc.SaveFileContent)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/create", OperationID: "CreateFile", Summary: "创建文件/目录", Description: "创建文件或目录", Tags: tag, Security: security}, fc.CreateFile)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/delete", OperationID: "DeleteFile", Summary: "删除文件/目录", Description: "删除文件或目录", Tags: tag, Security: security}, fc.DeleteFile)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/rename", OperationID: "RenameFile", Summary: "重命名文件", Description: "重命名文件", Tags: tag, Security: security}, fc.RenameFile)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/move", OperationID: "MoveFile", Summary: "移动文件", Description: "移动文件", Tags: tag, Security: security}, fc.MoveFile)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/copy", OperationID: "CopyFile", Summary: "复制文件", Description: "复制文件", Tags: tag, Security: security}, fc.CopyFile)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/files/download", OperationID: "DownloadFile", Summary: "下载单个文件", Description: "按 path 查询参数下载工作目录内的单个文件（流式）。", Tags: tag, Security: security}, fc.DownloadFileHuma)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/files/download-zip", OperationID: "DownloadZip", Summary: "批量下载为 Zip", Description: "按 path 查询参数（可重复）批量下载多个文件/目录，打包为 Zip 流。", Tags: tag, Security: security}, fc.DownloadZipHuma)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/upload", OperationID: "UploadArchive", Summary: "上传并解压归档", Description: "以 multipart/form-data 上传 zip/tar/gz/tgz 归档文件，并按 path 表单字段指定目录解压。", Tags: tag, Security: security}, fc.UploadArchiveHuma)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/files/uploadfiles", OperationID: "UploadFiles", Summary: "上传多个文件", Description: "以 multipart/form-data 上传多个文件（字段名 files，可选 paths 相对路径数组），支持保持文件夹结构。", Tags: tag, Security: security}, fc.UploadFilesHuma)
}

package controllers

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 文件管理
// ===========================================================================

// TAGetFileTreeOutput 获取文件树
type TAGetFileTreeOutput struct {
	Body utils.HumaResponse[[]*FileNode]
}

// TAGetFileTree 获取脚本目录的文件树
func (fc *FileController) TAGetFileTree(ctx context.Context, input *struct{}) (*TAGetFileTreeOutput, error) {
	root := &FileNode{
		Name:     filepath.Base(fc.workDir),
		Path:     "",
		IsDir:    true,
		Children: []*FileNode{},
	}

	if err := fc.buildFileTree(root); err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAGetFileTreeOutput{
		Body: utils.HumaResponse[[]*FileNode]{
			Code: 200,
			Msg:  "success",
			Data: root.Children,
		},
	}, nil
}

// TAGetFileContentInput 获取文件内容
type TAGetFileContentInput struct {
	Path string `query:"path" description:"文件路径"`
}

// TAGetFileContentOutput 获取文件内容
type TAGetFileContentOutput struct {
	Body utils.HumaResponse[struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}]
}

// TAGetFileContent 获取文件内容
func (fc *FileController) TAGetFileContent(ctx context.Context, input *TAGetFileContentInput) (*TAGetFileContentOutput, error) {
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

	return &TAGetFileContentOutput{
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

// TASaveFileContentInput 保存文件内容
type TASaveFileContentInput struct {
	Body struct {
		Path    string `json:"path" description:"文件路径"`
		Content string `json:"content" description:"文件内容"`
	}
}

// TASaveFileContentOutput 保存文件内容
type TASaveFileContentOutput struct {
	Body utils.HumaResponse[any]
}

// TASaveFileContent 保存文件内容
func (fc *FileController) TASaveFileContent(ctx context.Context, input *TASaveFileContentInput) (*TASaveFileContentOutput, error) {
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

	return &TASaveFileContentOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "保存成功",
		},
	}, nil
}

// TACreateFileInput 创建文件/目录
type TACreateFileInput struct {
	Body struct {
		Path  string `json:"path" description:"路径"`
		IsDir bool   `json:"isDir" description:"是否为目录"`
	}
}

// TACreateFileOutput 创建文件/目录
type TACreateFileOutput struct {
	Body utils.HumaResponse[any]
}

// TACreateFile 创建文件或目录
func (fc *FileController) TACreateFile(ctx context.Context, input *TACreateFileInput) (*TACreateFileOutput, error) {
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

	return &TACreateFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "创建成功",
		},
	}, nil
}

// TADeleteFileInput 删除文件/目录
type TADeleteFileInput struct {
	Body struct {
		Path string `json:"path" description:"路径"`
	}
}

// TADeleteFileOutput 删除文件/目录
type TADeleteFileOutput struct {
	Body utils.HumaResponse[any]
}

// TADeleteFile 删除文件或目录
func (fc *FileController) TADeleteFile(ctx context.Context, input *TADeleteFileInput) (*TADeleteFileOutput, error) {
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

	return &TADeleteFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TAMoveFileInput 移动文件
type TAMoveFileInput struct {
	Body struct {
		OldPath string `json:"oldPath" description:"源路径"`
		NewPath string `json:"newPath" description:"目标路径"`
	}
}

// TAMoveFileOutput 移动文件
type TAMoveFileOutput struct {
	Body utils.HumaResponse[any]
}

// TAMoveFile 移动文件
func (fc *FileController) TAMoveFile(ctx context.Context, input *TAMoveFileInput) (*TAMoveFileOutput, error) {
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
		return &TAMoveFileOutput{
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

	return &TAMoveFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TACopyFileInput 复制文件
type TACopyFileInput struct {
	Body struct {
		SourcePath string `json:"sourcePath" description:"源路径"`
		TargetPath string `json:"targetPath" description:"目标路径"`
	}
}

// TACopyFileOutput 复制文件
type TACopyFileOutput struct {
	Body utils.HumaResponse[any]
}

// TACopyFile 复制文件
func (fc *FileController) TACopyFile(ctx context.Context, input *TACopyFileInput) (*TACopyFileOutput, error) {
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
		return &TACopyFileOutput{
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

	return &TACopyFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// TARenameFileInput 重命名文件
type TARenameFileInput struct {
	Body struct {
		OldPath string `json:"oldPath" description:"旧路径"`
		NewPath string `json:"newPath" description:"新路径"`
	}
}

// TARenameFileOutput 重命名文件
type TARenameFileOutput struct {
	Body utils.HumaResponse[any]
}

// TARenameFile 重命名文件
func (fc *FileController) TARenameFile(ctx context.Context, input *TARenameFileInput) (*TARenameFileOutput, error) {
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
		return &TARenameFileOutput{
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

	return &TARenameFileOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "success",
		},
	}, nil
}

// RegisterAPIFileRoutes 注册 /api/v1 文件管理 Huma 路由
func (fc *FileController) RegisterAPIFileRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/files/tree",
		OperationID: "apiGetFileTree",
		Summary:     "获取文件树",
		Description: "获取脚本目录的文件树",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TAGetFileTree)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/files/content",
		OperationID: "apiGetFileContent",
		Summary:     "获取文件内容",
		Description: "获取指定文件的内容",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TAGetFileContent)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/files/content",
		OperationID: "apiSaveFileContent",
		Summary:     "保存文件内容",
		Description: "保存指定文件的内容",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TASaveFileContent)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/files/create",
		OperationID: "apiCreateFile",
		Summary:     "创建文件/目录",
		Description: "创建文件或目录",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TACreateFile)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/files/delete",
		OperationID: "apiDeleteFile",
		Summary:     "删除文件/目录",
		Description: "删除文件或目录",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TADeleteFile)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/files/rename",
		OperationID: "apiRenameFile",
		Summary:     "重命名文件",
		Description: "重命名文件",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TARenameFile)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/files/move",
		OperationID: "apiMoveFile",
		Summary:     "移动文件",
		Description: "移动文件",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TAMoveFile)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/files/copy",
		OperationID: "apiCopyFile",
		Summary:     "复制文件",
		Description: "复制文件",
		Tags:        []string{"文件管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, fc.TACopyFile)
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

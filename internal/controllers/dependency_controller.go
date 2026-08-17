package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/services/deps"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

type DependencyController struct {
	service *services.DependencyService
}

func NewDependencyController() *DependencyController {
	return &DependencyController{
		service: services.NewDependencyService(),
	}
}

// ===========================================================================
// 依赖管理业务方法
// ===========================================================================

// DependencyBody 依赖参数（通用）
type DependencyBody struct {
	Name        string `json:"name" description:"依赖包名"`
	Version     string `json:"version" description:"版本"`
	Language    string `json:"language" description:"语言"`
	LangVersion string `json:"lang_version" description:"语言版本"`
	Remark      string `json:"remark" description:"备注"`
}

// ListInput 获取依赖列表
type ListInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// ListOutput 获取依赖列表
type ListOutput struct {
	Body utils.HumaResponse[[]*vo.DependencyVO]
}

// List 获取依赖列表
func (c *DependencyController) List(ctx context.Context, input *ListInput) (*ListOutput, error) {
	depsList, err := c.service.List(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError("获取依赖列表失败")
	}

	return &ListOutput{
		Body: utils.HumaResponse[[]*vo.DependencyVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToDependencyVOListFromModels(depsList),
		},
	}, nil
}

// CreateInput 添加依赖
type CreateInput struct {
	Body DependencyBody
}

// CreateOutput 添加依赖
type CreateOutput struct {
	Body utils.HumaResponse[*vo.DependencyVO]
}

// Create 添加依赖
func (c *DependencyController) Create(ctx context.Context, input *CreateInput) (*CreateOutput, error) {
	req := input.Body
	if req.Name == "" || req.Language == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	dep := &models.Dependency{
		Name:        req.Name,
		Version:     req.Version,
		Language:    req.Language,
		LangVersion: req.LangVersion,
		Remark:      req.Remark,
	}

	if err := c.service.Create(dep); err != nil {
		return nil, utils.HumaBadRequest(err.Error())
	}

	return &CreateOutput{
		Body: utils.HumaResponse[*vo.DependencyVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToDependencyVO(dep),
		},
	}, nil
}

// DeleteInput 删除依赖
type DeleteInput struct {
	ID string `path:"id" description:"依赖ID"`
}

// DeleteOutput 删除依赖
type DeleteOutput struct {
	Body utils.HumaResponse[any]
}

// Delete 删除依赖
func (c *DependencyController) Delete(ctx context.Context, input *DeleteInput) (*DeleteOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.service.Delete(input.ID); err != nil {
		return nil, utils.HumaServerError("删除失败")
	}

	return &DeleteOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// InstallInput 安装依赖
type InstallInput struct {
	Body        DependencyBody
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// InstallOutput 安装依赖
type InstallOutput struct {
	Body utils.HumaResponse[any]
}

// Install 安装依赖
func (c *DependencyController) Install(ctx context.Context, input *InstallInput) (*InstallOutput, error) {
	req := input.Body
	if req.Name == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	language := req.Language
	if language == "" {
		language = input.Language
	}
	langVersion := req.LangVersion
	if langVersion == "" {
		langVersion = input.LangVersion
	}

	dep := &models.Dependency{
		Name:        req.Name,
		Version:     req.Version,
		Language:    language,
		LangVersion: langVersion,
		Remark:      req.Remark,
	}

	err := c.service.Install(dep)
	c.service.Create(dep)

	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &InstallOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "安装成功",
		},
	}, nil
}

// GetInstallCommandInput 获取安装命令
type GetInstallCommandInput struct {
	Body        DependencyBody
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// GetInstallCommandOutput 获取安装命令
type GetInstallCommandOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// GetInstallCommand 获取安装命令
func (c *DependencyController) GetInstallCommand(ctx context.Context, input *GetInstallCommandInput) (*GetInstallCommandOutput, error) {
	req := input.Body
	if req.Name == "" {
		return nil, utils.HumaBadRequest("参数错误")
	}

	language := req.Language
	if language == "" {
		language = input.Language
	}
	langVersion := req.LangVersion
	if langVersion == "" {
		langVersion = input.LangVersion
	}

	dep := &models.Dependency{
		Name:        req.Name,
		Version:     req.Version,
		Language:    language,
		LangVersion: langVersion,
	}

	cmd, err := c.service.GetInstallCommand(dep)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &GetInstallCommandOutput{
		Body: utils.HumaResponse[struct {
			Command string `json:"command"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Command string `json:"command"`
			}{Command: cmd},
		},
	}, nil
}

// UninstallInput 卸载依赖
type UninstallInput struct {
	ID    string `path:"id" description:"依赖ID"`
	Force bool   `query:"force" description:"是否强制卸载"`
}

// UninstallOutput 卸载依赖
type UninstallOutput struct {
	Body utils.HumaResponse[any]
}

// Uninstall 卸载依赖
func (c *DependencyController) Uninstall(ctx context.Context, input *UninstallInput) (*UninstallOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	depsList, _ := c.service.List("", "")
	var dep *models.Dependency
	for i := range depsList {
		if depsList[i].ID == input.ID {
			dep = &depsList[i]
			break
		}
	}

	if dep == nil {
		return nil, utils.HumaNotFound("依赖不存在")
	}

	if err := c.service.Uninstall(dep); err != nil {
		if !input.Force {
			return nil, utils.HumaServerError(err.Error())
		}
	}

	c.service.Delete(input.ID)

	return &UninstallOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "卸载成功",
		},
	}, nil
}

// ReinstallInput 重新安装依赖
type ReinstallInput struct {
	ID string `path:"id" description:"依赖ID"`
}

// ReinstallOutput 重新安装依赖
type ReinstallOutput struct {
	Body utils.HumaResponse[any]
}

// Reinstall 重新安装依赖
func (c *DependencyController) Reinstall(ctx context.Context, input *ReinstallInput) (*ReinstallOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	depsList, _ := c.service.List("", "")
	var dep *models.Dependency
	for i := range depsList {
		if depsList[i].ID == input.ID {
			dep = &depsList[i]
			break
		}
	}

	if dep == nil {
		return nil, utils.HumaNotFound("依赖不存在")
	}

	err := c.service.Install(dep)
	c.service.Create(dep)

	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &ReinstallOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "重新安装成功",
		},
	}, nil
}

// ReinstallAllInput 重新安装所有依赖
type ReinstallAllInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// ReinstallAllOutput 重新安装所有依赖
type ReinstallAllOutput struct {
	Body utils.HumaResponse[any]
}

// ReinstallAll 重新安装所有依赖
func (c *DependencyController) ReinstallAll(ctx context.Context, input *ReinstallAllInput) (*ReinstallAllOutput, error) {
	if input.Language == "" {
		return nil, utils.HumaBadRequest("缺少 language 参数")
	}

	depsList, err := c.service.List(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError("获取依赖列表失败")
	}

	var failed []string
	for i := range depsList {
		d := &depsList[i]
		err := c.service.Install(d)
		if err != nil {
			failed = append(failed, d.Name)
		}
		c.service.Create(d)
	}

	if len(failed) > 0 {
		return nil, utils.HumaServerError("部分包安装失败: " + strings.Join(failed, ", "))
	}

	return &ReinstallAllOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "全部重新安装成功",
		},
	}, nil
}

// ReinstallAllCmdInput 获取全部重装命令
type ReinstallAllCmdInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// ReinstallAllCmdOutput 获取全部重装命令
type ReinstallAllCmdOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// ReinstallAllCmd 获取全部重装命令
func (c *DependencyController) ReinstallAllCmd(ctx context.Context, input *ReinstallAllCmdInput) (*ReinstallAllCmdOutput, error) {
	if input.Language == "" {
		return nil, utils.HumaBadRequest("缺少 language 参数")
	}

	cmd, err := c.service.GetReinstallAllCommand(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &ReinstallAllCmdOutput{
		Body: utils.HumaResponse[struct {
			Command string `json:"command"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Command string `json:"command"`
			}{Command: cmd},
		},
	}, nil
}

// BatchInstallCmdInput 获取批量安装命令
type BatchInstallCmdInput struct {
	Body struct {
		Items []struct {
			Name        string `json:"name" description:"依赖包名"`
			Version     string `json:"version" description:"版本"`
			Language    string `json:"language" description:"语言"`
			LangVersion string `json:"lang_version" description:"语言版本"`
		} `json:"items" description:"依赖项列表"`
	}
}

// BatchInstallCmdOutput 获取批量安装命令
type BatchInstallCmdOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// BatchInstallCmd 获取批量安装命令
func (c *DependencyController) BatchInstallCmd(ctx context.Context, input *BatchInstallCmdInput) (*BatchInstallCmdOutput, error) {
	if len(input.Body.Items) == 0 {
		return nil, utils.HumaBadRequest("参数错误: items 不能为空且必须包含 name 和 language")
	}

	var depsList []models.Dependency
	for _, item := range input.Body.Items {
		depsList = append(depsList, models.Dependency{
			Name:        item.Name,
			Version:     item.Version,
			Language:    item.Language,
			LangVersion: item.LangVersion,
		})
	}

	cmd, err := c.service.GetBatchInstallCommand(depsList)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &BatchInstallCmdOutput{
		Body: utils.HumaResponse[struct {
			Command string `json:"command"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Command string `json:"command"`
			}{Command: cmd},
		},
	}, nil
}

// ParseAndImportInput 解析并导入依赖清单
type ParseAndImportInput struct {
	Body struct {
		Language    string `json:"language" description:"语言"`
		LangVersion string `json:"lang_version" description:"语言版本"`
		Content     string `json:"content" description:"清单文件内容"`
		ImportDB    bool   `json:"import_db" description:"是否持久化到数据库"`
	}
}

// ParseAndImportOutput 解析并导入依赖清单
type ParseAndImportOutput struct {
	Body utils.HumaResponse[struct {
		Dependencies []*vo.DependencyVO `json:"dependencies"`
		Command      string             `json:"command"`
	}]
}

// ParseAndImport 解析并导入依赖清单
func (c *DependencyController) ParseAndImport(ctx context.Context, input *ParseAndImportInput) (*ParseAndImportOutput, error) {
	req := input.Body
	if req.Language == "" || req.Content == "" {
		return nil, utils.HumaBadRequest("参数错误: language 和 content 必填")
	}

	parsedDeps, err := deps.ParseManifest(req.Language, req.Content)
	if err != nil {
		return nil, utils.HumaServerError("清单文件解析失败: " + err.Error())
	}

	if len(parsedDeps) == 0 {
		return nil, utils.HumaBadRequest("未解析到任何有效依赖包")
	}

	for i := range parsedDeps {
		parsedDeps[i].Language = req.Language
		parsedDeps[i].LangVersion = req.LangVersion
	}

	var finalDeps []models.Dependency
	if req.ImportDB {
		imported, err := c.service.ImportDependencies(parsedDeps)
		if err != nil {
			return nil, utils.HumaServerError("导入依赖记录至数据库失败: " + err.Error())
		}
		finalDeps = imported
	} else {
		finalDeps = parsedDeps
	}

	cmd, err := c.service.GetBatchInstallCommand(finalDeps)
	if err != nil {
		return nil, utils.HumaServerError("生成安装命令失败: " + err.Error())
	}

	return &ParseAndImportOutput{
		Body: utils.HumaResponse[struct {
			Dependencies []*vo.DependencyVO `json:"dependencies"`
			Command      string             `json:"command"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Dependencies []*vo.DependencyVO `json:"dependencies"`
				Command      string             `json:"command"`
			}{
				Dependencies: vo.ToDependencyVOListFromModels(finalDeps),
				Command:      cmd,
			},
		},
	}, nil
}

// GetInstalledInput 获取已安装的包
type GetInstalledInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// GetInstalledOutput 获取已安装的包
type GetInstalledOutput struct {
	Body utils.HumaResponse[[]models.Dependency]
}

// GetInstalled 获取已安装的包
func (c *DependencyController) GetInstalled(ctx context.Context, input *GetInstalledInput) (*GetInstalledOutput, error) {
	if input.Language == "" {
		return nil, utils.HumaBadRequest("缺少 language 参数")
	}

	packages, err := c.service.GetInstalledPackages(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError("获取已安装包失败: " + err.Error())
	}

	return &GetInstalledOutput{
		Body: utils.HumaResponse[[]models.Dependency]{
			Code: 200,
			Msg:  "success",
			Data: packages,
		},
	}, nil
}

// GetDepInstallCommandInput 获取自动补全的命令
type GetDepInstallCommandInput struct {
	LogID string `query:"log_id" description:"日志ID"`
}

// GetDepInstallCommandOutput 获取自动补全的命令
type GetDepInstallCommandOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// GetDepInstallCommand 获取自动补全的命令
func (c *DependencyController) GetDepInstallCommand(ctx context.Context, input *GetDepInstallCommandInput) (*GetDepInstallCommandOutput, error) {
	if input.LogID == "" {
		return nil, utils.HumaBadRequest("参数错误: log_id 不能为空")
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "baihu"
	}

	cmdStr := fmt.Sprintf("%q depinstall %s", execPath, input.LogID)

	return &GetDepInstallCommandOutput{
		Body: utils.HumaResponse[struct {
			Command string `json:"command"`
		}]{
			Code: 200,
			Msg:  "success",
			Data: struct {
				Command string `json:"command"`
			}{Command: cmdStr},
		},
	}, nil
}

// ===========================================================================
// 路由注册
// ===========================================================================

// RegisterAPIDependencyRoutes 注册 /api/v1 依赖管理 Huma 路由
func (c *DependencyController) RegisterAPIDependencyRoutes(api huma.API) {
	security := []map[string][]string{{"CookieAuth": {}}}
	tag := []string{"依赖管理"}

	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/deps", OperationID: "List", Summary: "获取依赖列表", Description: "获取依赖列表", Tags: tag, Security: security}, c.List)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps", OperationID: "Create", Summary: "添加依赖", Description: "添加依赖记录", Tags: tag, Security: security}, c.Create)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/deps/{id}", OperationID: "Delete", Summary: "删除依赖", Description: "删除依赖记录", Tags: tag, Security: security}, c.Delete)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/install", OperationID: "Install", Summary: "安装依赖", Description: "安装指定的依赖包", Tags: tag, Security: security}, c.Install)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/install-cmd", OperationID: "GetInstallCommand", Summary: "获取安装命令", Description: "获取指定依赖包的安装命令", Tags: tag, Security: security}, c.GetInstallCommand)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/uninstall/{id}", OperationID: "Uninstall", Summary: "卸载依赖", Description: "卸载指定的依赖包", Tags: tag, Security: security}, c.Uninstall)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/reinstall/{id}", OperationID: "Reinstall", Summary: "重新安装依赖", Description: "重新安装指定的依赖包", Tags: tag, Security: security}, c.Reinstall)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/reinstall-all", OperationID: "ReinstallAll", Summary: "重新安装所有依赖", Description: "重新安装指定语言的所有依赖", Tags: tag, Security: security}, c.ReinstallAll)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/reinstall-all-cmd", OperationID: "ReinstallAllCmd", Summary: "获取全部重装命令", Description: "获取指定语言的全部重装命令", Tags: tag, Security: security}, c.ReinstallAllCmd)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/batch-install-cmd", OperationID: "BatchInstallCmd", Summary: "获取批量安装命令", Description: "获取一批依赖包的合并安装命令", Tags: tag, Security: security}, c.BatchInstallCmd)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/deps/import", OperationID: "ParseAndImport", Summary: "解析并导入依赖清单", Description: "解析上传/粘贴的清单文件内容并批量导入至数据库", Tags: tag, Security: security}, c.ParseAndImport)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/deps/installed", OperationID: "GetInstalled", Summary: "获取已安装的包", Description: "获取指定语言已安装的包列表", Tags: tag, Security: security}, c.GetInstalled)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/deps/install-suggest-cmd", OperationID: "GetDepInstallCommand", Summary: "获取自动补全的命令", Description: "获取自动补全的安装命令", Tags: tag, Security: security}, c.GetDepInstallCommand)
}

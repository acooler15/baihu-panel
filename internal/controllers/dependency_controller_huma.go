package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/models/vo"
	"github.com/engigu/baihu-panel/internal/services/deps"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/danielgtaylor/huma/v2"
)

// ===========================================================================
// /api/v1 管理接口 —— 依赖管理
// ===========================================================================

// TADependencyInput 依赖参数（通用）
type TADependencyBody struct {
	Name        string `json:"name" description:"依赖包名"`
	Version     string `json:"version" description:"版本"`
	Language    string `json:"language" description:"语言"`
	LangVersion string `json:"lang_version" description:"语言版本"`
	Remark      string `json:"remark" description:"备注"`
}

// TAListInput 获取依赖列表
type TAListInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// TAListOutput 获取依赖列表
type TAListOutput struct {
	Body utils.HumaResponse[[]*vo.DependencyVO]
}

// TAList 获取依赖列表
func (c *DependencyController) TAList(ctx context.Context, input *TAListInput) (*TAListOutput, error) {
	depsList, err := c.service.List(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError("获取依赖列表失败")
	}

	return &TAListOutput{
		Body: utils.HumaResponse[[]*vo.DependencyVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToDependencyVOListFromModels(depsList),
		},
	}, nil
}

// TACreateInput 添加依赖
type TACreateInput struct {
	Body TADependencyBody
}

// TACreateOutput 添加依赖
type TACreateOutput struct {
	Body utils.HumaResponse[*vo.DependencyVO]
}

// TACreate 添加依赖
func (c *DependencyController) TACreate(ctx context.Context, input *TACreateInput) (*TACreateOutput, error) {
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

	return &TACreateOutput{
		Body: utils.HumaResponse[*vo.DependencyVO]{
			Code: 200,
			Msg:  "success",
			Data: vo.ToDependencyVO(dep),
		},
	}, nil
}

// TADeleteInput 删除依赖
type TADeleteInput struct {
	ID string `path:"id" description:"依赖ID"`
}

// TADeleteOutput 删除依赖
type TADeleteOutput struct {
	Body utils.HumaResponse[any]
}

// TADelete 删除依赖
func (c *DependencyController) TADelete(ctx context.Context, input *TADeleteInput) (*TADeleteOutput, error) {
	if input.ID == "" {
		return nil, utils.HumaBadRequest("无效的 ID")
	}

	if err := c.service.Delete(input.ID); err != nil {
		return nil, utils.HumaServerError("删除失败")
	}

	return &TADeleteOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "删除成功",
		},
	}, nil
}

// TAInstallInput 安装依赖
type TAInstallInput struct {
	Body        TADependencyBody
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// TAInstallOutput 安装依赖
type TAInstallOutput struct {
	Body utils.HumaResponse[any]
}

// TAInstall 安装依赖
func (c *DependencyController) TAInstall(ctx context.Context, input *TAInstallInput) (*TAInstallOutput, error) {
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

	return &TAInstallOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "安装成功",
		},
	}, nil
}

// TAGetInstallCommandInput 获取安装命令
type TAGetInstallCommandInput struct {
	Body        TADependencyBody
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// TAGetInstallCommandOutput 获取安装命令
type TAGetInstallCommandOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// TAGetInstallCommand 获取安装命令
func (c *DependencyController) TAGetInstallCommand(ctx context.Context, input *TAGetInstallCommandInput) (*TAGetInstallCommandOutput, error) {
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

	return &TAGetInstallCommandOutput{
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

// TAUninstallInput 卸载依赖
type TAUninstallInput struct {
	ID    string `path:"id" description:"依赖ID"`
	Force bool   `query:"force" description:"是否强制卸载"`
}

// TAUninstallOutput 卸载依赖
type TAUninstallOutput struct {
	Body utils.HumaResponse[any]
}

// TAUninstall 卸载依赖
func (c *DependencyController) TAUninstall(ctx context.Context, input *TAUninstallInput) (*TAUninstallOutput, error) {
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

	return &TAUninstallOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "卸载成功",
		},
	}, nil
}

// TAReinstallInput 重新安装依赖
type TAReinstallInput struct {
	ID string `path:"id" description:"依赖ID"`
}

// TAReinstallOutput 重新安装依赖
type TAReinstallOutput struct {
	Body utils.HumaResponse[any]
}

// TAReinstall 重新安装依赖
func (c *DependencyController) TAReinstall(ctx context.Context, input *TAReinstallInput) (*TAReinstallOutput, error) {
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

	return &TAReinstallOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "重新安装成功",
		},
	}, nil
}

// TAReinstallAllInput 重新安装所有依赖
type TAReinstallAllInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// TAReinstallAllOutput 重新安装所有依赖
type TAReinstallAllOutput struct {
	Body utils.HumaResponse[any]
}

// TAReinstallAll 重新安装所有依赖
func (c *DependencyController) TAReinstallAll(ctx context.Context, input *TAReinstallAllInput) (*TAReinstallAllOutput, error) {
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

	return &TAReinstallAllOutput{
		Body: utils.HumaResponse[any]{
			Code: 200,
			Msg:  "全部重新安装成功",
		},
	}, nil
}

// TAReinstallAllCmdInput 获取全部重装命令
type TAReinstallAllCmdInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// TAReinstallAllCmdOutput 获取全部重装命令
type TAReinstallAllCmdOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// TAReinstallAllCmd 获取全部重装命令
func (c *DependencyController) TAReinstallAllCmd(ctx context.Context, input *TAReinstallAllCmdInput) (*TAReinstallAllCmdOutput, error) {
	if input.Language == "" {
		return nil, utils.HumaBadRequest("缺少 language 参数")
	}

	cmd, err := c.service.GetReinstallAllCommand(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError(err.Error())
	}

	return &TAReinstallAllCmdOutput{
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

// TABatchInstallCmdInput 获取批量安装命令
type TABatchInstallCmdInput struct {
	Body struct {
		Items []struct {
			Name        string `json:"name" description:"依赖包名"`
			Version     string `json:"version" description:"版本"`
			Language    string `json:"language" description:"语言"`
			LangVersion string `json:"lang_version" description:"语言版本"`
		} `json:"items" description:"依赖项列表"`
	}
}

// TABatchInstallCmdOutput 获取批量安装命令
type TABatchInstallCmdOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// TABatchInstallCmd 获取批量安装命令
func (c *DependencyController) TABatchInstallCmd(ctx context.Context, input *TABatchInstallCmdInput) (*TABatchInstallCmdOutput, error) {
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

	return &TABatchInstallCmdOutput{
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

// TAParseAndImportInput 解析并导入依赖清单
type TAParseAndImportInput struct {
	Body struct {
		Language    string `json:"language" description:"语言"`
		LangVersion string `json:"lang_version" description:"语言版本"`
		Content     string `json:"content" description:"清单文件内容"`
		ImportDB    bool   `json:"import_db" description:"是否持久化到数据库"`
	}
}

// TAParseAndImportOutput 解析并导入依赖清单
type TAParseAndImportOutput struct {
	Body utils.HumaResponse[struct {
		Dependencies []*vo.DependencyVO `json:"dependencies"`
		Command      string             `json:"command"`
	}]
}

// TAParseAndImport 解析并导入依赖清单
func (c *DependencyController) TAParseAndImport(ctx context.Context, input *TAParseAndImportInput) (*TAParseAndImportOutput, error) {
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

	return &TAParseAndImportOutput{
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

// TAGetInstalledInput 获取已安装的包
type TAGetInstalledInput struct {
	Language    string `query:"language" description:"语言"`
	LangVersion string `query:"lang_version" description:"语言版本"`
}

// TAGetInstalledOutput 获取已安装的包
type TAGetInstalledOutput struct {
	Body utils.HumaResponse[[]models.Dependency]
}

// TAGetInstalled 获取已安装的包
func (c *DependencyController) TAGetInstalled(ctx context.Context, input *TAGetInstalledInput) (*TAGetInstalledOutput, error) {
	if input.Language == "" {
		return nil, utils.HumaBadRequest("缺少 language 参数")
	}

	packages, err := c.service.GetInstalledPackages(input.Language, input.LangVersion)
	if err != nil {
		return nil, utils.HumaServerError("获取已安装包失败: " + err.Error())
	}

	return &TAGetInstalledOutput{
		Body: utils.HumaResponse[[]models.Dependency]{
			Code: 200,
			Msg:  "success",
			Data: packages,
		},
	}, nil
}

// TAGetDepInstallCommandInput 获取自动补全的命令
type TAGetDepInstallCommandInput struct {
	LogID string `query:"log_id" description:"日志ID"`
}

// TAGetDepInstallCommandOutput 获取自动补全的命令
type TAGetDepInstallCommandOutput struct {
	Body utils.HumaResponse[struct {
		Command string `json:"command"`
	}]
}

// TAGetDepInstallCommand 获取自动补全的命令
func (c *DependencyController) TAGetDepInstallCommand(ctx context.Context, input *TAGetDepInstallCommandInput) (*TAGetDepInstallCommandOutput, error) {
	if input.LogID == "" {
		return nil, utils.HumaBadRequest("参数错误: log_id 不能为空")
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "baihu"
	}

	cmdStr := fmt.Sprintf("%q depinstall %s", execPath, input.LogID)

	return &TAGetDepInstallCommandOutput{
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

// RegisterAPIDependencyRoutes 注册 /api/v1 依赖管理 Huma 路由
func (c *DependencyController) RegisterAPIDependencyRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/deps",
		OperationID: "apiDepList",
		Summary:     "获取依赖列表",
		Description: "获取依赖列表",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAList)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps",
		OperationID: "apiDepCreate",
		Summary:     "添加依赖",
		Description: "添加依赖记录",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TACreate)

	huma.Register(api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/deps/{id}",
		OperationID: "apiDepDelete",
		Summary:     "删除依赖",
		Description: "删除依赖记录",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TADelete)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/install",
		OperationID: "apiDepInstall",
		Summary:     "安装依赖",
		Description: "安装指定的依赖包",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAInstall)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/install-cmd",
		OperationID: "apiDepGetInstallCommand",
		Summary:     "获取安装命令",
		Description: "获取指定依赖包的安装命令",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAGetInstallCommand)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/uninstall/{id}",
		OperationID: "apiDepUninstall",
		Summary:     "卸载依赖",
		Description: "卸载指定的依赖包",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAUninstall)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/reinstall/{id}",
		OperationID: "apiDepReinstall",
		Summary:     "重新安装依赖",
		Description: "重新安装指定的依赖包",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAReinstall)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/reinstall-all",
		OperationID: "apiDepReinstallAll",
		Summary:     "重新安装所有依赖",
		Description: "重新安装指定语言的所有依赖",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAReinstallAll)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/reinstall-all-cmd",
		OperationID: "apiDepReinstallAllCmd",
		Summary:     "获取全部重装命令",
		Description: "获取指定语言的全部重装命令",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAReinstallAllCmd)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/batch-install-cmd",
		OperationID: "apiDepBatchInstallCmd",
		Summary:     "获取批量安装命令",
		Description: "获取一批依赖包的合并安装命令",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TABatchInstallCmd)

	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/deps/import",
		OperationID: "apiDepParseAndImport",
		Summary:     "解析并导入依赖清单",
		Description: "解析上传/粘贴的清单文件内容并批量导入至数据库",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAParseAndImport)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/deps/installed",
		OperationID: "apiDepGetInstalled",
		Summary:     "获取已安装的包",
		Description: "获取指定语言已安装的包列表",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAGetInstalled)

	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/deps/install-suggest-cmd",
		OperationID: "apiDepInstallSuggestCmd",
		Summary:     "获取自动补全的命令",
		Description: "获取自动补全的安装命令",
		Tags:        []string{"依赖管理"},
		Security:    []map[string][]string{{"CookieAuth": {}}},
	}, c.TAGetDepInstallCommand)
}

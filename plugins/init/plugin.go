package main

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"tgp/core"
	"tgp/internal/cleanup"
	"tgp/plugins/init/generator"
)

//go:embed plugin.md
var pluginDoc string

// InitPlugin реализует интерфейс Plugin.
type InitPlugin struct{}

// Info возвращает информацию о плагине.
func (p *InitPlugin) Info() core.PluginInfo {
	return core.PluginInfo{
		Name:        "init",
		Persistent:  false,
		Version:     "2.4.0",
		Doc:         pluginDoc,
		Description: translate("Initialize new project with basic structure"),
		Author:      "seniorGolang",
		License:     "MIT",
		Category:    "init",
		Commands: []core.Command{
			{
				Path:        []string{"init"},
				Description: translate("Initialize new project with basic structure"),
				Options: []core.Option{
					{
						Name:        "module",
						Short:       "m",
						Type:        "string",
						Description: translate("Go module name"),
						Required:    false,
					},
					{
						Name:        "project",
						Short:       "p",
						Type:        "string",
						Description: translate("Project name"),
						Required:    true,
					},
					{
						Name:        "service",
						Short:       "s",
						Type:        "string",
						Description: translate("Service name"),
						Required:    false,
						Default:     "someService",
					},
					{
						Name:        "dir",
						Short:       "d",
						Type:        "string",
						Description: translate("Directory for project creation (default: ./<project>)"),
						Required:    false,
					},
				},
			},
		},
		AllowedHTTP:      []string{},
		AllowedShellCMDs: []string{"go"}, // Разрешаем выполнение команды go
	}
}

// Execute выполняет основную логику плагина.
func (p *InitPlugin) Execute(rootDir string, request core.Storage, path ...string) (response core.Storage, err error) {

	logger := core.GetLogger()

	logger.Info(translate("init plugin started"))

	// Получаем параметры из request
	projectNameVal, ok := request.Get("project")
	if !ok {
		return nil, fmt.Errorf("project option is required")
	}
	projectName, ok := projectNameVal.(string)
	if !ok || projectName == "" {
		return nil, fmt.Errorf("project option is required and must be a string")
	}

	moduleNameVal, _ := request.Get("module")
	moduleName, _ := moduleNameVal.(string)
	if moduleName == "" {
		moduleName = projectName
	}

	serviceNameVal, _ := request.Get("service")
	serviceName, _ := serviceNameVal.(string)
	if serviceName == "" {
		serviceName = "someService"
	}

	baseDirVal, _ := request.Get("dir")
	baseDir, _ := baseDirVal.(string)
	if baseDir == "" {
		baseDir = projectName
	}

	// В WASM файловая система монтируется в корень "/", поэтому используем относительные пути
	// rootDir уже является корнем файловой системы внутри WASM
	// baseDir должен быть относительным путем от rootDir
	if filepath.IsAbs(baseDir) {
		// Если передан абсолютный путь, вычисляем относительный путь от rootDir
		relPath, err := filepath.Rel(rootDir, baseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path from rootDir: %w", err)
		}
		baseDir = relPath
	}

	// Логирование
	logger.Info(fmt.Sprintf("initializing project: module=%s, project=%s, service=%s, baseDir=%s", moduleName, projectName, serviceName, baseDir))

	// Очищаем старые сгенерированные файлы перед новой генерацией (если директория существует)
	if err := cleanup.CleanupGeneratedFiles(baseDir); err != nil {
		logger.Warn(fmt.Sprintf("failed to cleanup generated files: error=%v", err))
		// Не возвращаем ошибку, так как очистка не критична
	}

	// Инициализируем проект
	if err = generator.GenerateSkeleton(moduleName, projectName, serviceName, baseDir); err != nil {
		logger.Error(fmt.Sprintf("failed to initialize project: error=%v", err))
		return nil, fmt.Errorf("initialize project: %w", err)
	}

	logger.Info(fmt.Sprintf("project initialized successfully: baseDir=%s", baseDir))

	// Создаем response
	response = core.NewStorage()
	if err = response.Set("baseDir", baseDir); err != nil {
		return nil, fmt.Errorf("failed to set response: %w", err)
	}

	return response, nil
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance core.Plugin = &InitPlugin{}

package main

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"tgp/internal/cleanup"
	"tgp/plugins/init/generator"
	"tgp/shared"
)

//go:embed plugin.md
var pluginDoc string

// InitPlugin реализует интерфейс Plugin.
type InitPlugin struct{}

// Info возвращает информацию о плагине.
func (p *InitPlugin) Info() shared.PluginInfo {
	return shared.PluginInfo{
		Name:        "init",
		Persistent:  false,
		Version:     "2.4.0",
		Doc:         pluginDoc,
		Description: translate("Initialize new project with basic structure"),
		Author:      "seniorGolang",
		License:     "MIT",
		Category:    "init",
		Commands: []shared.Command{
			{
				Path:        []string{"init"},
				Description: translate("Initialize new project with basic structure"),
				Options: []shared.Option{
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
func (p *InitPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {

	logger := shared.GetLogger()

	logger.Info(translate("init plugin started"))

	// Получаем параметры
	projectName, ok := options["project"].(string)
	if !ok || projectName == "" {
		return fmt.Errorf("project option is required and must be a string")
	}

	moduleName, _ := options["module"].(string)
	if moduleName == "" {
		moduleName = projectName
	}

	serviceName, _ := options["service"].(string)
	if serviceName == "" {
		serviceName = "someService"
	}

	baseDir, _ := options["dir"].(string)
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
			return fmt.Errorf("failed to compute relative path from rootDir: %w", err)
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
		return fmt.Errorf("initialize project: %w", err)
	}

	logger.Info(fmt.Sprintf("project initialized successfully: baseDir=%s", baseDir))

	return nil
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance shared.Plugin = &InitPlugin{}

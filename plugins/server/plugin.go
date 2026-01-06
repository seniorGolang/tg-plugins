package main

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"tgp/core"
	"tgp/internal/cleanup"
	"tgp/plugins/server/generator"
)

//go:embed plugin.md
var pluginDoc string

// ServerPlugin реализует интерфейс Plugin.
type ServerPlugin struct{}

// Info возвращает информацию о плагине.
func (p *ServerPlugin) Info() core.PluginInfo {
	return core.PluginInfo{
		Name:         "server",
		Version:      "2.4.0",
		Doc:          pluginDoc,
		Description:  translate("Server code generator for HTTP/JSON-RPC servers based on Fiber"),
		Author:       "AlexK (seniorGolang@gmail.com)",
		License:      "MIT",
		Category:     "server",
		Dependencies: []string{"astg"},
		Commands: []core.Command{
			{
				Path:        []string{"server"},
				Description: translate("Generate server code"),
				Options: []core.Option{
					{
						Name:        "contracts",
						Short:       "c",
						Type:        "string",
						Description: translate("Path to contracts folder (relative to rootDir)"),
						Required:    false,
						Default:     "contracts",
					},
					{
						Name:        "out",
						Short:       "o",
						Type:        "string",
						Description: translate("Path to output directory"),
						Required:    true,
					},
					{
						Name:        "ifaces",
						Type:        "string",
						Description: translate("Comma-separated list of interfaces for filtering (e.g., \"Contract1,Contract2\")"),
						Required:    false,
					},
					{
						Name:        "verbose",
						Short:       "v",
						Type:        "bool",
						Description: translate("Verbose output"),
						Required:    false,
						Default:     false,
					},
				},
			},
		},
	}
}

// Execute выполняет основную логику плагина.
func (p *ServerPlugin) Execute(rootDir string, request core.Storage, path ...string) (response core.Storage, err error) {

	logger := core.GetLogger()

	logger.Info(translate("server plugin started"))

	// Получаем project из request
	projectVal, ok := request.Get("project")
	if !ok {
		return nil, fmt.Errorf("project is required in request")
	}

	// Преобразуем project в parser.Project (используется tgp/internal/parser.Project)
	// projectVal может быть map[string]any (сериализованный Project)
	coreProject, err := generator.DeserializeProject(projectVal)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize project: %w", err)
	}

	// Получаем out из request
	outDirVal, ok := request.Get("out")
	if !ok {
		return nil, fmt.Errorf("out option is required")
	}
	outDir, ok := outDirVal.(string)
	if !ok || outDir == "" {
		return nil, fmt.Errorf("out option is required and must be a string")
	}

	// В WASM файловая система монтируется в корень "/", поэтому используем относительные пути
	// rootDir уже является корнем файловой системы внутри WASM
	// outDir должен быть относительным путем от rootDir
	if filepath.IsAbs(outDir) {
		// Если передан абсолютный путь, вычисляем относительный путь от rootDir
		relPath, err := filepath.Rel(rootDir, outDir)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path from rootDir: %w", err)
		}
		outDir = relPath
	}

	// projectRoot в WASM всегда является корнем файловой системы ("/")
	// Используем пустую строку или "." для обозначения корня
	projectRoot := "."

	// Получаем список интерфейсов для фильтрации
	var ifaces []string
	ifacesVal, ok := request.Get("ifaces")
	if ok {
		ifacesStr, ok := ifacesVal.(string)
		if ok && ifacesStr != "" {
			parts := strings.FieldsFunc(ifacesStr, func(r rune) bool {
				return r == ',' || r == ' ' || r == '\t'
			})
			ifaces = make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					ifaces = append(ifaces, part)
				}
			}
		}
	}

	// Устанавливаем verbose режим
	if verboseVal, ok := request.Get("verbose"); ok {
		if verbose, ok := verboseVal.(bool); ok && verbose {
			generator.SetVerbose(true)
		}
	}

	// Очищаем старые сгенерированные файлы перед новой генерацией
	if err := cleanup.CleanupGeneratedFiles(outDir); err != nil {
		logger.Warn(fmt.Sprintf("failed to cleanup generated files: error=%v", err))
		// Не возвращаем ошибку, так как очистка не критична
	}

	// Генерируем транспортные файлы
	logger.Info(fmt.Sprintf("generating transport files: outDir=%s ifaces=%v", outDir, ifaces))
	if err = generator.GenerateTransportFiles(coreProject, outDir, projectRoot, ifaces...); err != nil {
		logger.Error(fmt.Sprintf("failed to generate transport files: outDir=%s error=%v", outDir, err))
		return nil, err
	}

	// Генерируем сервер для каждого контракта
	for _, contract := range coreProject.Contracts {
		// Проверяем фильтры
		if len(ifaces) > 0 {
			found := false
			for _, ifaceName := range ifaces {
				if contract.Name == ifaceName || contract.ID == ifaceName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		logger.Info(fmt.Sprintf("generating server for contract: contract=%s", contract.ID))
		if err = generator.GenerateServer(coreProject, contract.ID, outDir, projectRoot); err != nil {
			logger.Error(fmt.Sprintf("failed to generate server: contract=%s error=%v", contract.ID, err))
			return nil, err
		}
		logger.Info(fmt.Sprintf("server generated successfully: contract=%s", contract.ID))
	}

	logger.Info(translate("server plugin completed"))

	// Создаем response
	response = core.NewStorage()
	if err = response.Set("outDir", outDir); err != nil {
		return nil, fmt.Errorf("failed to set response: %w", err)
	}

	return response, nil
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance core.Plugin = &ServerPlugin{}

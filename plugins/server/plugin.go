package main

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"tgp/plugins/server/generator"
	"tgp/shared"
)

//go:embed plugin.md
var pluginDoc string

// ServerPlugin реализует интерфейс Plugin.
type ServerPlugin struct{}

// Info возвращает информацию о плагине.
func (p *ServerPlugin) Info() shared.PluginInfo {
	return shared.PluginInfo{
		Name:        "server",
		Version:     "2.4.0",
		Doc:         pluginDoc,
		Description: "Генератор серверного кода для HTTP/JSON-RPC серверов на основе Fiber",
		Author:      "AlexK (seniorGolang@gmail.com)",
		License:     "MIT",
		Category:    "server",
		Commands: []shared.Command{
			{
				Path:        []string{"server"},
				Description: "Генерация серверного кода",
				Options: []shared.Option{
					{
						Name:        "contracts",
						Short:       "c",
						Type:        "string",
						Description: "Путь к папке с контрактами (относительно rootDir)",
						Required:    false,
						Default:     "contracts",
					},
					{
						Name:        "out",
						Short:       "o",
						Type:        "string",
						Description: "Путь к выходной директории",
						Required:    true,
					},
					{
						Name:        "ifaces",
						Type:        "string",
						Description: "Список интерфейсов через запятую для фильтрации (например: \"Contract1,Contract2\")",
						Required:    false,
					},
					{
						Name:        "verbose",
						Short:       "v",
						Type:        "bool",
						Description: "Подробный вывод",
						Required:    false,
						Default:     false,
					},
				},
			},
		},
	}
}

// Execute выполняет основную логику плагина.
func (p *ServerPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {

	logger := shared.GetLogger()

	logger.Info("server плагин запущен")

	// shared.Project это map[string]any, который содержит сериализованный core.Project
	// Преобразуем его напрямую в core.Project
	coreProject, err := generator.DeserializeProject(project)
	if err != nil {
		return fmt.Errorf("failed to deserialize project: %w", err)
	}

	// Получаем out из опций
	outDir, ok := options["out"].(string)
	if !ok || outDir == "" {
		return fmt.Errorf("out option is required and must be a string")
	}

	// В WASM файловая система монтируется в корень "/", поэтому используем относительные пути
	// rootDir уже является корнем файловой системы внутри WASM
	// outDir должен быть относительным путем от rootDir
	if filepath.IsAbs(outDir) {
		// Если передан абсолютный путь, вычисляем относительный путь от rootDir
		relPath, err := filepath.Rel(rootDir, outDir)
		if err != nil {
			return fmt.Errorf("failed to compute relative path from rootDir: %w", err)
		}
		outDir = relPath
	}

	// projectRoot в WASM всегда является корнем файловой системы ("/")
	// Используем пустую строку или "." для обозначения корня
	projectRoot := "."

	// Получаем список интерфейсов для фильтрации
	var ifaces []string
	if ifacesStr, ok := options["ifaces"].(string); ok && ifacesStr != "" {
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

	// Устанавливаем verbose режим
	if verbose, ok := options["verbose"].(bool); ok && verbose {
		generator.SetVerbose(true)
	}

	// Генерируем транспортные файлы
	logger.Info(fmt.Sprintf("generating transport files: outDir=%s ifaces=%v", outDir, ifaces))
	if err = generator.GenerateTransportFiles(coreProject, outDir, projectRoot, ifaces...); err != nil {
		logger.Error(fmt.Sprintf("failed to generate transport files: outDir=%s error=%v", outDir, err))
		return
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
			return
		}
		logger.Info(fmt.Sprintf("server generated successfully: contract=%s", contract.ID))
	}

	logger.Info("server плагин завершён")
	return
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance shared.Plugin = &ServerPlugin{}

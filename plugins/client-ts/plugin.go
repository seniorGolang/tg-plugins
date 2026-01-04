package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tgp/internal/cleanup"
	"tgp/plugins/client-ts/generator"
	"tgp/shared"
)

// ClientTsPlugin реализует интерфейс Plugin.
type ClientTsPlugin struct{}

// Info возвращает информацию о плагине.
func (p *ClientTsPlugin) Info() shared.PluginInfo {
	return shared.PluginInfo{
		Name:        "client-ts",
		Persistent:  false,
		Version:     "2.4.0",
		Description: translate("TypeScript client generator for HTTP/JSON-RPC servers"),
		Author:      "AlexK (seniorGolang@gmail.com)",
		License:     "MIT",
		Category:    "client",
		Commands: []shared.Command{
			{
				Path:        []string{"client", "ts"},
				Description: translate("Generate TypeScript client"),
				Options: []shared.Option{
					{
						Name:        "out",
						Short:       "o",
						Type:        "string",
						Description: translate("Path to output directory"),
						Required:    true,
					},
					{
						Name:        "contracts",
						Short:       "c",
						Type:        "string",
						Description: translate("Comma-separated list of contracts for filtering (e.g., \"Contract1,Contract2\")"),
						Required:    false,
					},
					{
						Name:        "doc-file",
						Type:        "string",
						Description: translate("Path to documentation file (default: <out>/README.md)"),
						Required:    false,
					},
					{
						Name:        "no-doc",
						Type:        "bool",
						Description: translate("Disable documentation generation"),
						Required:    false,
						Default:     false,
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
func (p *ClientTsPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {

	logger := shared.GetLogger()

	logger.Info(translate("client-ts plugin started"))

	// Преобразуем shared.Project в shared.Project (десериализация)
	coreProject, err := generator.ConvertFromSharedProject(project)
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

	// Создаем выходную директорию
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Получаем опции документации
	docOpts := generator.DocOptions{
		Enabled: true,
	}

	if noDoc, ok := options["no-doc"].(bool); ok && noDoc {
		docOpts.Enabled = false
	}

	if docFile, ok := options["doc-file"].(string); ok && docFile != "" {
		docOpts.FilePath = docFile
	} else if docOpts.Enabled {
		docOpts.FilePath = filepath.Join(outDir, "README.md")
	}

	// Получаем список контрактов для фильтрации
	var contracts []string
	if contractsStr, ok := options["contracts"].(string); ok && contractsStr != "" {
		contracts = strings.Split(contractsStr, ",")
		for i, contract := range contracts {
			contracts[i] = strings.TrimSpace(contract)
		}
	}

	// Фильтруем контракты, если указаны
	if len(contracts) > 0 {
		filteredContracts := make([]*shared.Contract, 0)
		for _, contract := range coreProject.Contracts {
			for _, filterName := range contracts {
				if contract.Name == filterName || contract.ID == filterName {
					filteredContracts = append(filteredContracts, contract)
					break
				}
			}
		}
		coreProject.Contracts = filteredContracts
	}

	// Очищаем старые сгенерированные файлы перед новой генерацией
	if err := cleanup.CleanupGeneratedFiles(outDir); err != nil {
		logger.Warn(fmt.Sprintf("failed to cleanup generated files: error=%v", err))
		// Не возвращаем ошибку, так как очистка не критична
	}

	// Генерируем клиент
	if err := generator.GenerateClient(coreProject, outDir, rootDir, docOpts); err != nil {
		logger.Error(fmt.Sprintf("failed to generate TypeScript client: error=%v", err))
		return fmt.Errorf("generate TypeScript client: %w", err)
	}

	logger.Info(translate("client-ts plugin completed"))
	return nil
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance shared.Plugin = &ClientTsPlugin{}

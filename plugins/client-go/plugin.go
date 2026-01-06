package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tgp/core"
	"tgp/internal/cleanup"
	"tgp/plugins/client-go/generator"
)

// ClientGoPlugin реализует интерфейс Plugin.
type ClientGoPlugin struct{}

// Info возвращает информацию о плагине.
func (p *ClientGoPlugin) Info() core.PluginInfo {
	return core.PluginInfo{
		Name:         "client-go",
		Persistent:   false,
		Version:      "2.4.0",
		Description:  translate("Go client generator for HTTP/JSON-RPC servers"),
		Author:       "AlexK (seniorGolang@gmail.com)",
		License:      "MIT",
		Category:     "client",
		Dependencies: []string{"astg"},
		Commands: []core.Command{
			{
				Path:        []string{"client", "go"},
				Description: translate("Generate Go client"),
				Options: []core.Option{
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
func (p *ClientGoPlugin) Execute(rootDir string, request core.Storage, path ...string) (response core.Storage, err error) {

	logger := core.GetLogger()

	logger.Info(translate("client-go plugin started"))

	// Получаем project из request
	projectVal, ok := request.Get("project")
	if !ok {
		return nil, fmt.Errorf("project is required in request")
	}

	// Преобразуем project в core.Project
	coreProject, err := generator.ConvertFromSharedProject(projectVal)
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

	// Создаем выходную директорию
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Получаем опции документации
	docOpts := generator.DocOptions{
		Enabled: true,
	}

	noDocVal, ok := request.Get("no-doc")
	if ok {
		if noDoc, ok := noDocVal.(bool); ok && noDoc {
			docOpts.Enabled = false
		}
	}

	docFileVal, ok := request.Get("doc-file")
	if ok {
		if docFile, ok := docFileVal.(string); ok && docFile != "" {
			docOpts.FilePath = docFile
		} else if docOpts.Enabled {
			docOpts.FilePath = filepath.Join(outDir, "README.md")
		}
	} else if docOpts.Enabled {
		docOpts.FilePath = filepath.Join(outDir, "README.md")
	}

	// Получаем список контрактов для фильтрации
	var contracts []string
	contractsVal, ok := request.Get("contracts")
	if ok {
		if contractsStr, ok := contractsVal.(string); ok && contractsStr != "" {
			contracts = strings.Split(contractsStr, ",")
			for i, contract := range contracts {
				contracts[i] = strings.TrimSpace(contract)
			}
		}
	}

	// Фильтруем контракты, если указаны
	if len(contracts) > 0 {
		filteredContracts := make([]*core.Contract, 0)
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
		logger.Error(fmt.Sprintf("failed to generate Go client: error=%v", err))
		return nil, fmt.Errorf("generate Go client: %w", err)
	}

	logger.Info(translate("client-go plugin completed"))

	// Создаем response
	response = core.NewStorage()
	if err = response.Set("outDir", outDir); err != nil {
		return nil, fmt.Errorf("failed to set response: %w", err)
	}

	return response, nil
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance core.Plugin = &ClientGoPlugin{}

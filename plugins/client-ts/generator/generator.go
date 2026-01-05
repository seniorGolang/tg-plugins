// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package generator

import (
	"encoding/json"
	"fmt"

	"tgp/core"
	"tgp/plugins/client-ts/renderer"
)

// DeserializeProject десериализует Project из JSON.
func DeserializeProject(projectData interface{}) (*core.Project, error) {

	projectBytes, err := json.Marshal(projectData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal project data: %w", err)
	}

	var project core.Project
	if err := json.Unmarshal(projectBytes, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project: %w", err)
	}

	return &project, nil
}

// ConvertFromSharedProject конвертирует core.Project в core.Project (алиас для совместимости).
func ConvertFromSharedProject(sharedProject interface{}) (*core.Project, error) {
	return DeserializeProject(sharedProject)
}

// DocOptions содержит опции для генерации документации
type DocOptions struct {
	Enabled  bool   // Включена ли генерация документации (по умолчанию true)
	FilePath string // Полный путь к файлу документации (пусто = outDir/README.md)
}

// GenerateClient генерирует клиент для всех контрактов.
func GenerateClient(project *core.Project, outDir, projectRoot string, docOpts DocOptions) error {

	logger := core.GetLogger()
	logger.Info(fmt.Sprintf("generating TypeScript client: outDir=%s", outDir))

	gen := &generator{
		project:     project,
		outDir:      outDir,
		projectRoot: projectRoot,
		renderer:    renderer.NewClientRenderer(project, outDir, projectRoot),
	}

	if err := gen.generate(docOpts); err != nil {
		logger.Error(fmt.Sprintf("failed to generate TypeScript client: error=%v", err))
		return err
	}

	logger.Info("TypeScript client generated successfully")
	return nil
}

type generator struct {
	project     *core.Project
	outDir      string
	projectRoot string
	renderer    *renderer.ClientRenderer
}

// contains проверяет, содержится ли ключ в map.
func (g *generator) contains(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func (g *generator) generate(docOpts DocOptions) error {

	// Генерируем базовые файлы клиента один раз для всех контрактов
	if g.renderer.HasJsonRPC() || g.renderer.HasHTTP() {
		if err := g.renderer.RenderClientOptions(); err != nil {
			return err
		}
		if err := g.renderer.RenderVersion(); err != nil {
			return err
		}
		// Генерируем JSON-RPC библиотеку перед генерацией клиента
		if g.renderer.HasJsonRPC() {
			if err := g.renderer.RenderJsonRPCLibrary(); err != nil {
				return err
			}
		}
		if err := g.renderer.RenderClient(); err != nil {
			return err
		}
		if err := g.renderer.RenderClientError(); err != nil {
			return err
		}
		if g.renderer.HasJsonRPC() {
			if err := g.renderer.RenderClientBatch(); err != nil {
				return err
			}
		}
	}

	// Собираем typeID типов из всех контрактов перед генерацией
	allCollectedTypeIDs := make(map[string]bool)
	for _, contract := range g.project.Contracts {
		if g.contains(contract.Annotations, renderer.TagServerJsonRPC) || g.contains(contract.Annotations, renderer.TagServerHTTP) {
			contractTypeIDs := g.renderer.CollectTypeIDsForExchange(contract)
			// Объединяем typeID из всех контрактов
			for typeID := range contractTypeIDs {
				allCollectedTypeIDs[typeID] = true
			}
		}
	}

	// Генерируем локальные версии типов один раз для всех контрактов
	// ВАЖНО: для TS нужно генерировать ВСЕ типы, включая внешние либы
	if len(allCollectedTypeIDs) > 0 {
		if err := g.renderer.RenderClientTypes(allCollectedTypeIDs); err != nil {
			return err
		}
	}

	// Генерируем клиент для каждого контракта
	for _, contract := range g.project.Contracts {
		if g.contains(contract.Annotations, renderer.TagServerJsonRPC) || g.contains(contract.Annotations, renderer.TagServerHTTP) {
			// Генерируем exchange для клиента
			if err := g.renderer.RenderExchangeTypes(contract); err != nil {
				return err
			}
			// Генерируем JSON-RPC клиент
			if g.contains(contract.Annotations, renderer.TagServerJsonRPC) {
				if err := g.renderer.RenderJsonRPCClientClass(contract); err != nil {
					return err
				}
			}
			// Генерируем HTTP клиент
			if g.contains(contract.Annotations, renderer.TagServerHTTP) {
				if err := g.renderer.RenderHTTPClientClass(contract); err != nil {
					return err
				}
			}
		}
	}

	// Генерируем tsconfig.json для IDE поддержки
	if g.renderer.HasJsonRPC() || g.renderer.HasHTTP() {
		if err := g.renderer.RenderTsConfig(); err != nil {
			return err
		}
	}

	// Генерируем документацию
	if docOpts.Enabled && (g.renderer.HasJsonRPC() || g.renderer.HasHTTP()) {
		rendererDocOpts := renderer.DocOptions{
			Enabled:  docOpts.Enabled,
			FilePath: docOpts.FilePath,
		}
		if err := g.renderer.RenderReadmeTS(rendererDocOpts); err != nil {
			return err
		}
	}

	return nil
}

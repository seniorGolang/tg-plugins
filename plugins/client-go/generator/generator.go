// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package generator

import (
	"encoding/json"
	"fmt"

	"tgp/plugins/client-go/renderer"
	"tgp/shared"
)

// DeserializeProject десериализует Project из JSON.
func DeserializeProject(projectData interface{}) (*shared.Project, error) {

	projectBytes, err := json.Marshal(projectData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal project data: %w", err)
	}

	var project shared.Project
	if err := json.Unmarshal(projectBytes, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project: %w", err)
	}

	return &project, nil
}

// ConvertFromSharedProject конвертирует shared.Project в shared.Project (алиас для совместимости).
func ConvertFromSharedProject(sharedProject interface{}) (*shared.Project, error) {
	return DeserializeProject(sharedProject)
}

// DocOptions содержит опции для генерации документации
type DocOptions struct {
	Enabled  bool   // Включена ли генерация документации (по умолчанию true)
	FilePath string // Полный путь к файлу документации (пусто = outDir/README.md)
}

// GenerateClient генерирует клиент для всех контрактов.
func GenerateClient(project *shared.Project, outDir, projectRoot string, docOpts DocOptions) error {

	logger := shared.GetLogger()
	logger.Info(fmt.Sprintf("generating Go client: outDir=%s", outDir))

	gen := &generator{
		project:     project,
		outDir:      outDir,
		projectRoot: projectRoot,
		renderer:    renderer.NewClientRenderer(project, outDir, projectRoot),
	}

	if err := gen.generate(docOpts); err != nil {
		logger.Error(fmt.Sprintf("failed to generate Go client: error=%v", err))
		return err
	}

	logger.Info("Go client generated successfully")
	return nil
}

type generator struct {
	project     *shared.Project
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
	if len(allCollectedTypeIDs) > 0 {
		if err := g.renderer.RenderClientTypes(allCollectedTypeIDs); err != nil {
			return err
		}
	}

	// Генерируем клиент для каждого контракта
	for _, contract := range g.project.Contracts {
		if g.contains(contract.Annotations, renderer.TagServerJsonRPC) || g.contains(contract.Annotations, renderer.TagServerHTTP) {
			// Генерируем exchange для клиента
			if err := g.renderer.RenderExchange(contract); err != nil {
				return err
			}
			// Генерируем service-client
			if err := g.renderer.RenderServiceClient(contract); err != nil {
				return err
			}
			// Генерируем метрики, если нужно
			if g.renderer.HasMetrics() && g.contains(contract.Annotations, renderer.TagMetrics) {
				if err := g.renderer.RenderClientMetrics(); err != nil {
					return err
				}
			}
		}
	}

	// Генерируем документацию
	if docOpts.Enabled && (g.renderer.HasJsonRPC() || g.renderer.HasHTTP()) {
		if err := g.renderer.RenderReadmeGo(docOpts); err != nil {
			return err
		}
	}

	return nil
}


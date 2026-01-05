// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package generator

import (
	"encoding/json"
	"fmt"

	tgpcore "tgp/core"
	"tgp/plugins/server/core"
	"tgp/plugins/server/renderer"
	"tgp/plugins/server/utils"
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

// GenerateServer генерирует код сервера для указанного контракта.
func GenerateServer(project *core.Project, contractID string, outDir, projectRoot string) error {

	if err := utils.ValidateProject(project); err != nil {
		return fmt.Errorf("invalid project: %w", err)
	}
	if err := utils.ValidateContractID(contractID); err != nil {
		return fmt.Errorf("invalid contractID: %w", err)
	}
	if err := utils.ValidateOutDir(outDir); err != nil {
		return fmt.Errorf("invalid outDir: %w", err)
	}

	contract, err := utils.FindContract(project, contractID)
	if err != nil {
		return fmt.Errorf("find contract: %w", err)
	}

	if err := utils.ValidateContract(contract, project); err != nil {
		return fmt.Errorf("validate contract: %w", err)
	}

	logger := tgpcore.GetLogger()
	resetStats()
	setupCacheLogger()
	renderer.SetOnFileSaved(onFileSaved)
	logger.Info(fmt.Sprintf("generating server: contract=%s outDir=%s", contractID, outDir))

	gen := &generator{
		project:  project,
		contract: contract,
		outDir:   outDir,
		renderer: renderer.NewContractRenderer(project, contract, outDir, projectRoot),
	}

	if err := gen.generate(); err != nil {
		logger.Error(fmt.Sprintf("failed to generate server: contract=%s error=%v", contractID, err))
		return err
	}

	logStats()
	logger.Info(fmt.Sprintf("server generated successfully: contract=%s", contractID))
	return nil
}

// GenerateTransportFiles генерирует транспортные файлы верхнего уровня один раз для всех контрактов.
func GenerateTransportFiles(project *core.Project, outDir, projectRoot string, contracts ...string) error {

	if err := utils.ValidateProject(project); err != nil {
		return fmt.Errorf("invalid project: %w", err)
	}
	if err := utils.ValidateOutDir(outDir); err != nil {
		return fmt.Errorf("invalid outDir: %w", err)
	}

	logger := tgpcore.GetLogger()
	resetStats()
	setupCacheLogger()
	renderer.SetOnFileSaved(onFileSaved)
	logger.Info(fmt.Sprintf("generating transport files: outDir=%s contracts=%v", outDir, contracts))

	gen := &generator{
		project:  project,
		outDir:   outDir,
		renderer: renderer.NewTransportRenderer(project, outDir, projectRoot),
	}

	if len(contracts) > 0 {
		filteredProject, err := filterContracts(project, contracts)
		if err != nil {
			return fmt.Errorf("filter contracts: %w", err)
		}
		gen.project = filteredProject
		gen.renderer = renderer.NewTransportRenderer(filteredProject, outDir, projectRoot)
	}

	if err := gen.generateTransport(); err != nil {
		logger.Error(fmt.Sprintf("failed to generate transport files: outDir=%s error=%v", outDir, err))
		return err
	}

	logStats()
	logger.Info(fmt.Sprintf("transport files generated successfully: outDir=%s", outDir))
	return nil
}

// generator содержит состояние генератора сервера.
type generator struct {
	project  *core.Project
	contract *core.Contract
	outDir   string
	renderer renderer.Renderer
}

// generate генерирует все файлы для контракта.
func (g *generator) generate() error {

	logVerbose("rendering HTTP: contract=%s", g.contract.ID)
	if err := g.renderer.RenderHTTP(); err != nil {
		return fmt.Errorf("render HTTP: %w", err)
	}

	logVerbose("rendering server: contract=%s", g.contract.ID)
	if err := g.renderer.RenderServer(); err != nil {
		return fmt.Errorf("render server: %w", err)
	}

	logVerbose("rendering exchange: contract=%s", g.contract.ID)
	if err := g.renderer.RenderExchange(); err != nil {
		return fmt.Errorf("render exchange: %w", err)
	}

	logVerbose("rendering middleware: contract=%s", g.contract.ID)
	if err := g.renderer.RenderMiddleware(); err != nil {
		return fmt.Errorf("render middleware: %w", err)
	}

	if g.contract.Annotations.Contains("trace") {
		logVerbose("rendering trace: contract=%s", g.contract.ID)
		if err := g.renderer.RenderTrace(); err != nil {
			return fmt.Errorf("render trace: %w", err)
		}
	}
	if g.contract.Annotations.Contains("metrics") {
		logVerbose("rendering metrics: contract=%s", g.contract.ID)
		if err := g.renderer.RenderMetrics(); err != nil {
			return fmt.Errorf("render metrics: %w", err)
		}
	}
	if g.contract.Annotations.Contains("log") {
		logVerbose("rendering logger: contract=%s", g.contract.ID)
		if err := g.renderer.RenderLogger(); err != nil {
			return fmt.Errorf("render logger: %w", err)
		}
	}
	if g.contract.Annotations.Contains("jsonRPC-server") {
		logVerbose("rendering JSON-RPC: contract=%s", g.contract.ID)
		if err := g.renderer.RenderJsonRPC(); err != nil {
			return fmt.Errorf("render JSON-RPC: %w", err)
		}
	}
	if g.contract.Annotations.Contains("http-server") {
		logVerbose("rendering REST: contract=%s", g.contract.ID)
		if err := g.renderer.RenderREST(); err != nil {
			return fmt.Errorf("render REST: %w", err)
		}
	}

	return nil
}

// generateTransport генерирует транспортные файлы.
func (g *generator) generateTransport() error {

	logVerbose("rendering transport HTTP")
	if err := g.renderer.RenderTransportHTTP(); err != nil {
		return fmt.Errorf("render transport HTTP: %w", err)
	}

	logVerbose("rendering transport context")
	if err := g.renderer.RenderTransportContext(); err != nil {
		return fmt.Errorf("render transport context: %w", err)
	}

	logVerbose("rendering transport logger")
	if err := g.renderer.RenderTransportLogger(); err != nil {
		return fmt.Errorf("render transport logger: %w", err)
	}

	logVerbose("rendering transport fiber")
	if err := g.renderer.RenderTransportFiber(); err != nil {
		return fmt.Errorf("render transport fiber: %w", err)
	}

	logVerbose("rendering transport header")
	if err := g.renderer.RenderTransportHeader(); err != nil {
		return fmt.Errorf("render transport header: %w", err)
	}

	logVerbose("rendering transport errors")
	if err := g.renderer.RenderTransportErrors(); err != nil {
		return fmt.Errorf("render transport errors: %w", err)
	}

	logVerbose("rendering transport server")
	if err := g.renderer.RenderTransportServer(); err != nil {
		return fmt.Errorf("render transport server: %w", err)
	}

	logVerbose("rendering transport options")
	if err := g.renderer.RenderTransportOptions(); err != nil {
		return fmt.Errorf("render transport options: %w", err)
	}

	logVerbose("rendering transport metrics")
	if err := g.renderer.RenderTransportMetrics(); err != nil {
		return fmt.Errorf("render transport metrics: %w", err)
	}

	logVerbose("rendering transport version")
	if err := g.renderer.RenderTransportVersion(); err != nil {
		return fmt.Errorf("render transport version: %w", err)
	}

	if g.hasJsonRPC() {
		logVerbose("rendering transport JSON-RPC")
		if err := g.renderer.RenderTransportJsonRPC(); err != nil {
			return fmt.Errorf("render transport JSON-RPC: %w", err)
		}
	}

	return nil
}

// filterContracts фильтрует контракты по указанным именам или ID.
func filterContracts(project *core.Project, contractNames []string) (*core.Project, error) {

	contractMap := make(map[string]bool, len(contractNames))
	for _, name := range contractNames {
		contractMap[name] = true
	}

	filteredContracts := make([]*core.Contract, 0)
	for _, contract := range project.Contracts {
		if contractMap[contract.Name] || contractMap[contract.ID] {
			filteredContracts = append(filteredContracts, contract)
		}
	}

	filteredProject := *project
	filteredProject.Contracts = filteredContracts
	return &filteredProject, nil
}

// hasJsonRPC проверяет, есть ли контракты с JSON-RPC.
func (g *generator) hasJsonRPC() bool {

	for _, contract := range g.project.Contracts {
		if contract.Annotations.Contains("jsonRPC-server") {
			return true
		}
	}
	return false
}

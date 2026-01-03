// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"tgp/plugins/server/core"
)

//go:embed pkg/context pkg/logger pkg/tracer pkg/viewer
var pkgFiles embed.FS

// baseRenderer содержит общую функциональность для всех рендереров.
type baseRenderer struct {
	project     *core.Project
	contract    *core.Contract
	outDir      string
	projectRoot string
}

// newBaseRenderer создает базовый рендерер.
func newBaseRenderer(project *core.Project, contract *core.Contract, outDir, projectRoot string) *baseRenderer {
	return &baseRenderer{
		project:     project,
		contract:    contract,
		outDir:      outDir,
		projectRoot: projectRoot,
	}
}

// pkgPath возвращает путь пакета для указанной директории.
func (r *baseRenderer) pkgPath(dir string) string {

	// В WASM файловая система монтируется в корень "/", поэтому используем относительные пути
	// dir уже является относительным путем от rootDir
	// projectRoot в WASM всегда "." (корень файловой системы)
	// Преобразуем относительный путь в путь пакета
	pkgDir := filepath.ToSlash(dir)

	// Убираем ведущий "./" если есть
	pkgDir = strings.TrimPrefix(pkgDir, "./")

	// Если pkgDir не пустой, добавляем "/" в начало для формирования пути пакета
	if pkgDir != "" && !strings.HasPrefix(pkgDir, "/") {
		pkgDir = "/" + pkgDir
	}

	return r.project.ModulePath + pkgDir
}

// pkgCopyTo копирует встроенные пакеты в выходную директорию.
func (r *baseRenderer) pkgCopyTo(pkg, dst string) (err error) {

	pkgPath := path.Join("pkg", pkg)
	var entries []fs.DirEntry
	if entries, err = pkgFiles.ReadDir(pkgPath); err != nil {
		return
	}
	for _, entry := range entries {
		var fileContent []byte
		if fileContent, err = pkgFiles.ReadFile(fmt.Sprintf("%s/%s", pkgPath, entry.Name())); err != nil {
			return
		}
		if err = os.MkdirAll(path.Join(dst, pkg), 0700); err != nil {
			return
		}
		filename := path.Join(dst, pkg, entry.Name())
		if err = os.WriteFile(filename, fileContent, 0600); err != nil {
			return
		}
	}
	return
}

// hasJsonRPC проверяет, есть ли контракты с JSON-RPC.
func (r *baseRenderer) hasJsonRPC() bool {

	for _, contract := range r.project.Contracts {
		if contract.Annotations.Contains(TagServerJsonRPC) {
			return true
		}
	}
	return false
}

// hasHTTPService проверяет, есть ли контракты с HTTP сервисом.
func (r *baseRenderer) hasHTTPService() bool {

	for _, contract := range r.project.Contracts {
		if contract.Annotations.Contains(TagServerHTTP) {
			return true
		}
	}
	return false
}

// hasMetrics проверяет, есть ли контракты с метриками.
func (r *baseRenderer) hasMetrics() bool {

	for _, contract := range r.project.Contracts {
		if contract.Annotations.Contains(TagMetrics) {
			return true
		}
	}
	return false
}

// hasTrace проверяет, есть ли контракты с трейсингом.
func (r *baseRenderer) hasTrace() bool {

	for _, contract := range r.project.Contracts {
		if contract.Annotations.Contains(TagTrace) {
			return true
		}
	}
	return false
}

// contractRenderer рендерер для конкретного контракта.
type contractRenderer struct {
	*baseRenderer
}

// NewContractRenderer создает рендерер для конкретного контракта.
func NewContractRenderer(project *core.Project, contract *core.Contract, outDir, projectRoot string) Renderer {
	return &contractRenderer{
		baseRenderer: newBaseRenderer(project, contract, outDir, projectRoot),
	}
}

// transportRenderer рендерер для транспортных файлов (генерируются один раз).
type transportRenderer struct {
	*baseRenderer
}

// NewTransportRenderer создает рендерер для транспортных файлов.
func NewTransportRenderer(project *core.Project, outDir, projectRoot string) Renderer {
	return &transportRenderer{
		baseRenderer: newBaseRenderer(project, nil, outDir, projectRoot),
	}
}

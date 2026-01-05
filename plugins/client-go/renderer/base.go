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

	"tgp/core"
)

//go:embed pkg/jsonrpc
var pkgFiles embed.FS

// ClientRenderer содержит общую функциональность для генерации клиента.
type ClientRenderer struct {
	project     *core.Project
	outDir      string
	projectRoot string
}

// NewClientRenderer создает новый рендерер клиента.
func NewClientRenderer(project *core.Project, outDir, projectRoot string) *ClientRenderer {
	return &ClientRenderer{
		project:     project,
		outDir:      outDir,
		projectRoot: projectRoot,
	}
}

// pkgPath возвращает путь пакета для указанной директории.
func (r *ClientRenderer) pkgPath(dir string) string {

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
func (r *ClientRenderer) pkgCopyTo(pkg, dst string) (err error) {

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

// HasJsonRPC проверяет, есть ли контракты с JSON-RPC.
func (r *ClientRenderer) HasJsonRPC() bool {

	for _, contract := range r.project.Contracts {
		if r.contains(contract.Annotations, TagServerJsonRPC) {
			return true
		}
	}
	return false
}

// HasHTTP проверяет, есть ли контракты с HTTP.
func (r *ClientRenderer) HasHTTP() bool {

	for _, contract := range r.project.Contracts {
		if r.contains(contract.Annotations, TagServerHTTP) {
			return true
		}
	}
	return false
}

// HasMetrics проверяет, есть ли контракты с метриками.
func (r *ClientRenderer) HasMetrics() bool {

	for _, contract := range r.project.Contracts {
		if r.contains(contract.Annotations, TagMetrics) {
			return true
		}
	}
	return false
}

// contains проверяет, содержится ли ключ в map.
func (r *ClientRenderer) contains(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// Все методы Render* реализованы в соответствующих файлах:
// - RenderClientOptions - options.go
// - RenderVersion - version.go
// - RenderClient - client.go
// - RenderClientError - error.go
// - RenderClientBatch - batch.go
// - CollectTypeIDsForExchange - collector.go
// - RenderClientTypes - types.go
// - RenderExchange - exchange.go
// - RenderServiceClient - service.go
// - RenderClientMetrics - metrics.go
// - RenderReadmeGo - readme.go

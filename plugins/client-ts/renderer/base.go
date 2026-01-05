// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strings"
	"unicode"

	"tgp/core"
)

// Для TS клиента jsonrpc библиотека генерируется через шаблоны, а не копируется
// Поэтому pkgFiles не используется для TS

// ClientRenderer содержит общую функциональность для генерации клиента.
type ClientRenderer struct {
	project     *core.Project
	outDir      string
	projectRoot string
	contract    *core.Contract
	knownTypes  map[string]int
	typeDefTs   map[string]typeDefTs
}

// NewClientRenderer создает новый рендерер клиента.
func NewClientRenderer(project *core.Project, outDir, projectRoot string) *ClientRenderer {
	return &ClientRenderer{
		project:     project,
		outDir:      outDir,
		projectRoot: projectRoot,
		knownTypes:  make(map[string]int),
		typeDefTs:   make(map[string]typeDefTs),
	}
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

// contains проверяет, содержится ли ключ в map.
func (r *ClientRenderer) contains(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// isTypeFromCurrentProject проверяет, является ли тип из текущего проекта.
func (r *ClientRenderer) isTypeFromCurrentProject(importPkgPath string) bool {
	// Если ImportPkgPath начинается с ModulePath проекта, это тип из текущего проекта
	if r.project.ModulePath != "" && strings.HasPrefix(importPkgPath, r.project.ModulePath) {
		return true
	}
	return false
}

// tsFileName возвращает имя файла для TypeScript (в snake_case).
func (r *ClientRenderer) tsFileName(contract *core.Contract) string {
	name := contract.Name
	if len(name) == 0 {
		return ""
	}
	// Преобразуем в snake_case: добавляем подчеркивание перед заглавными буквами
	// Для случаев типа "HTTPService" -> "http_service", "JsonRPCService" -> "json_rpc_service", "JWKS" -> "jwks"
	result := make([]rune, 0, len(name)*2)
	for i, r := range name {
		if i > 0 {
			prevR := rune(name[i-1])
			// Добавляем подчеркивание перед заглавной буквой, если предыдущая была маленькой
			if unicode.IsUpper(r) && unicode.IsLower(prevR) {
				result = append(result, '_')
			}
			// Добавляем подчеркивание перед последней заглавной буквой в последовательности заглавных,
			// если следующая буква маленькая
			if unicode.IsUpper(r) && unicode.IsUpper(prevR) {
				if i+1 < len(name) && unicode.IsLower(rune(name[i+1])) {
					result = append(result, '_')
				}
			}
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// lcName возвращает имя в lowerCamelCase.
func (r *ClientRenderer) lcName(s string) string {
	if len(s) == 0 {
		return ""
	}
	// Используем правильную функцию для преобразования в lowerCamelCase
	return toLowerCamel(s)
}

// requestTypeName возвращает имя типа request для метода.
func (r *ClientRenderer) requestTypeName(contract *core.Contract, method *core.Method) string {
	return fmt.Sprintf("Request%s%s", contract.Name, method.Name)
}

// responseTypeName возвращает имя типа response для метода.
func (r *ClientRenderer) responseTypeName(contract *core.Contract, method *core.Method) string {
	return fmt.Sprintf("Response%s%s", contract.Name, method.Name)
}

// toLowerCamel конвертирует строку в lowerCamelCase.
func toLowerCamel(s string) string {
	if s == "" {
		return s
	}
	// Если все символы заглавные, возвращаем как есть
	isAllUpper := true
	for _, v := range s {
		if v >= 'a' && v <= 'z' {
			isAllUpper = false
			break
		}
	}
	if isAllUpper {
		return s
	}
	// Преобразуем первую букву в нижний регистр
	if len(s) > 0 {
		first := rune(s[0])
		if first >= 'A' && first <= 'Z' {
			s = strings.ToLower(string(first)) + s[1:]
		}
	}
	// Убираем подчеркивания и преобразуем в camelCase
	parts := strings.Split(s, "_")
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(string(parts[i][0])) + parts[i][1:]
		}
	}
	return result
}

// Все методы Render* реализованы в соответствующих файлах:
// - RenderClientOptions - options.go
// - RenderVersion - version.go
// - RenderClient - client.go
// - RenderClientError - error.go
// - RenderClientBatch - batch.go
// - CollectTypeIDsForExchange - collector.go
// - RenderClientTypes - types.go
// - RenderExchangeTypes - exchange.go
// - RenderJsonRPCClientClass - jsonrpc-client.go
// - RenderHTTPClientClass - http-client.go
// - RenderReadmeTS - readme.go
// - RenderTsConfig - tsconfig.go

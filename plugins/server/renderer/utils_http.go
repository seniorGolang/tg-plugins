// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
)

// batchPath возвращает путь для batch запросов.
func (r *contractRenderer) batchPath() string {

	prefix := r.contract.Annotations.Value(TagHttpPrefix, "")
	pathValue := r.contract.Annotations.Value(TagHttpPath, "/"+toLowerCamel(r.contract.Name))
	if prefix != "" {
		return "/" + prefix + pathValue
	}
	return pathValue
}

// methodHTTPMethod возвращает HTTP метод для метода контракта.
func (r *contractRenderer) methodHTTPMethod(method *core.Method) string {

	httpMethod := method.Annotations.Value(TagMethodHTTP, "")
	if httpMethod == "" {
		return "post"
	}
	switch strings.ToUpper(httpMethod) {
	case "GET":
		return "get"
	case "PUT":
		return "put"
	case "PATCH":
		return "patch"
	case "DELETE":
		return "delete"
	case "OPTIONS":
		return "options"
	default:
		return "post"
	}
}

// methodHTTPPath возвращает HTTP путь для метода контракта.
func (r *contractRenderer) methodHTTPPath(method *core.Method) string {

	prefix := r.contract.Annotations.Value(TagHttpPrefix, "")
	methodPath := method.Annotations.Value(TagHttpPath, "/"+toLowerCamel(r.contract.Name)+"/"+toLowerCamel(method.Name))
	if prefix != "" {
		return "/" + prefix + methodPath
	}
	return methodPath
}

// methodJsonRPCPath возвращает путь для JSON-RPC метода.
func (r *contractRenderer) methodJsonRPCPath(method *core.Method) string {

	var elements []string
	elements = append(elements, "/")
	prefix := r.contract.Annotations.Value(TagHttpPrefix, "")
	urlPath := method.Annotations.Value(TagHttpPath, "/"+toLowerCamel(r.contract.Name)+"/"+toLowerCamel(method.Name))
	urlPath = strings.Split(urlPath, ":")[0]
	return path.Join(append(elements, prefix, urlPath)...)
}

// methodIsJsonRPC проверяет, является ли метод JSON-RPC методом.
func (r *contractRenderer) methodIsJsonRPC(method *core.Method) bool {

	if method == nil || method.Annotations == nil {
		return false
	}
	return r.contract != nil && r.contract.Annotations.Contains(TagServerJsonRPC) && !method.Annotations.Contains(TagMethodHTTP)
}

// methodHandlerQual возвращает квалифицированное имя обработчика для метода.
func (r *contractRenderer) methodHandlerQual(srcFile *GoFile, method *core.Method) Code {

	handlerValue := method.Annotations.Value(TagHandler, "")
	if handlerValue == "" {
		return Id("")
	}
	if tokens := strings.Split(handlerValue, ":"); len(tokens) == 2 {
		pkgPath := tokens[0]
		funcName := tokens[1]
		baseName := filepath.Base(pkgPath)
		srcFile.ImportName(pkgPath, baseName)
		return Qual(pkgPath, funcName)
	}
	return Id(handlerValue)
}

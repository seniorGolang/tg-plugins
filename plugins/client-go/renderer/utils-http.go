// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"context"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/core"
)

// varHeaderMap возвращает маппинг переменных на HTTP заголовки.
func (r *ClientRenderer) varHeaderMap(method *core.Method) map[string]string {
	headers := make(map[string]string)
	if httpHeaders, ok := method.Annotations[TagHttpHeader]; ok && httpHeaders != "" {
		headerPairs := strings.Split(httpHeaders, ",")
		for _, pair := range headerPairs {
			if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
				arg := strings.TrimSpace(pairTokens[0])
				header := strings.TrimSpace(pairTokens[1])
				headers[arg] = header
			}
		}
	}
	return headers
}

// varCookieMap возвращает маппинг переменных на HTTP cookies.
func (r *ClientRenderer) varCookieMap(method *core.Method) map[string]string {
	cookies := make(map[string]string)
	if httpCookies, ok := method.Annotations[TagHttpCookies]; ok && httpCookies != "" {
		cookiePairs := strings.Split(httpCookies, ",")
		for _, pair := range cookiePairs {
			if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
				arg := strings.TrimSpace(pairTokens[0])
				cookie := strings.TrimSpace(pairTokens[1])
				cookies[arg] = cookie
			}
		}
	}
	return cookies
}

// argPathMap возвращает маппинг аргументов на path параметры.
func (r *ClientRenderer) argPathMap(method *core.Method) map[string]string {
	paths := make(map[string]string)
	if urlPath, ok := method.Annotations[TagHttpPath]; ok && urlPath != "" {
		urlTokens := strings.Split(urlPath, "/")
		for _, token := range urlTokens {
			if strings.HasPrefix(token, ":") {
				arg := strings.TrimSpace(strings.TrimPrefix(token, ":"))
				paths[arg] = arg
			}
		}
	}
	return paths
}

// argParamMap возвращает маппинг аргументов на query параметры.
func (r *ClientRenderer) argParamMap(method *core.Method) map[string]string {
	params := make(map[string]string)
	if urlArgs, ok := method.Annotations[TagHttpArg]; ok && urlArgs != "" {
		paramPairs := strings.Split(urlArgs, ",")
		for _, pair := range paramPairs {
			if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
				arg := strings.TrimSpace(pairTokens[0])
				param := strings.TrimSpace(pairTokens[1])
				params[arg] = param
			}
		}
	}
	return params
}

// argByName находит аргумент по имени.
func (r *ClientRenderer) argByName(method *core.Method, argName string) *core.Variable {
	argName = strings.TrimPrefix(argName, "!")
	for _, arg := range method.Args {
		if arg.Name == argName {
			return arg
		}
	}
	return nil
}

// varToString генерирует код для конвертации переменной в строку.
func (r *ClientRenderer) varToString(ctx context.Context, variable *core.Variable) Code {
	// Проверяем, является ли тип строкой
	if variable.TypeID == "string" {
		return Id(ToLowerCamel(variable.Name))
	}
	// Для остальных типов используем fmt.Sprint
	return Qual(PackageFmt, "Sprint").Call(Id(ToLowerCamel(variable.Name)))
}

// contractNameToLowerCamel возвращает имя контракта в lowerCamelCase.
func (r *ClientRenderer) contractNameToLowerCamel(contract *core.Contract) string {
	if contract == nil {
		return ""
	}
	return ToLowerCamel(contract.Name)
}

// methodNameToLowerCamel возвращает имя метода в lowerCamelCase.
func (r *ClientRenderer) methodNameToLowerCamel(method *core.Method) string {
	if method == nil {
		return ""
	}
	return ToLowerCamel(method.Name)
}

// contractNameToLower возвращает имя контракта в lowercase (для JSON-RPC).
func (r *ClientRenderer) contractNameToLower(contract *core.Contract) string {
	if contract == nil {
		return ""
	}
	return strings.ToLower(contract.Name)
}

// methodNameToLower возвращает имя метода в lowercase (для JSON-RPC).
func (r *ClientRenderer) methodNameToLower(method *core.Method) string {
	if method == nil {
		return ""
	}
	return strings.ToLower(method.Name)
}

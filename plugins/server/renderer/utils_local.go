// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"strings"
	"unicode"

	"tgp/internal/parser"
)

// requestStructName возвращает имя структуры request для метода.
func requestStructName(contractName string, methodName string) string {

	return "request" + contractName + methodName
}

// responseStructName возвращает имя структуры response для метода.
func responseStructName(contractName string, methodName string) string {

	return "response" + contractName + methodName
}

// argsWithoutContext возвращает аргументы без первого context.Context, если он есть.
func argsWithoutContext(method *parser.Method) []*parser.Variable {

	if len(method.Args) == 0 {
		return method.Args
	}
	if method.Args[0].TypeID == "context:Context" {
		return method.Args[1:]
	}
	return method.Args
}

// resultsWithoutError возвращает результаты без последнего error, если он есть.
func resultsWithoutError(method *parser.Method) []*parser.Variable {

	if len(method.Results) == 0 {
		return method.Results
	}
	if method.Results[len(method.Results)-1].TypeID == "error" {
		return method.Results[:len(method.Results)-1]
	}
	return method.Results
}

// removeSkippedFields удаляет поля из списка, которые указаны в skipFields.
func removeSkippedFields(vars []*parser.Variable, skipFields []string) []*parser.Variable {

	result := make([]*parser.Variable, 0, len(vars))
	for _, v := range vars {
		found := false
		for _, skip := range skipFields {
			if v.Name == skip {
				found = true
				break
			}
		}
		if !found {
			result = append(result, v)
		}
	}
	return result
}

// toCamel конвертирует строку в CamelCase.
func toCamel(s string) string {

	if s == "" {
		return s
	}
	s = strings.Trim(s, " ")
	result := strings.Builder{}
	capNext := true
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			result.WriteRune(r)
			capNext = false
		case r >= '0' && r <= '9':
			result.WriteRune(r)
			capNext = false
		case r >= 'a' && r <= 'z':
			if capNext {
				result.WriteRune(unicode.ToUpper(r))
			} else {
				result.WriteRune(r)
			}
			capNext = false
		case r == '_' || r == ' ' || r == '-' || r == '.':
			capNext = true
		default:
			result.WriteRune(r)
			capNext = false
		}
	}
	return result.String()
}

// toLowerCamel конвертирует строку в lowerCamelCase.
func toLowerCamel(s string) string {

	if s == "" {
		return s
	}
	// Проверяем, все ли символы в верхнем регистре
	allUpper := true
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			allUpper = false
			break
		}
	}
	if allUpper {
		return s
	}
	// Если первый символ в верхнем регистре, делаем его нижним
	runes := []rune(s)
	if len(runes) > 0 && runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] = unicode.ToLower(runes[0])
		s = string(runes)
	}
	// Применяем toCamel, но с capNext = false
	s = strings.Trim(s, " ")
	result := strings.Builder{}
	capNext := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			result.WriteRune(r)
			capNext = false
		case r >= '0' && r <= '9':
			result.WriteRune(r)
			capNext = false
		case r >= 'a' && r <= 'z':
			if capNext {
				result.WriteRune(unicode.ToUpper(r))
			} else {
				result.WriteRune(r)
			}
			capNext = false
		case r == '_' || r == ' ' || r == '-' || r == '.':
			capNext = true
		default:
			result.WriteRune(r)
			capNext = false
		}
	}
	return result.String()
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package utils

import (
	"slices"

	"tgp/internal/parser"
)

// IsContextFirst проверяет, является ли первый аргумент context.Context.
func IsContextFirst(vars []*parser.Variable) bool {

	if len(vars) == 0 {
		return false
	}
	// Проверяем по typeID - если это "context:Context", то это context.Context
	return vars[0].TypeID == "context:Context"
}

// IsErrorLast проверяет, является ли последний результат error.
func IsErrorLast(vars []*parser.Variable) bool {

	if len(vars) == 0 {
		return false
	}
	return vars[len(vars)-1].TypeID == "error"
}

// ArgsWithoutContext возвращает аргументы без первого context.Context, если он есть.
func ArgsWithoutContext(method *parser.Method) []*parser.Variable {

	if IsContextFirst(method.Args) {
		return method.Args[1:]
	}
	return method.Args
}

// ArgsFieldsWithoutContext возвращает аргументы без context, обрабатывая inline поля.
// Примечание: Полная обработка inline полей реализована в renderer.inline_fields.go
// Эта функция используется только в контексте, где inline поля не требуются.
func ArgsFieldsWithoutContext(method *parser.Method) []*parser.Variable {

	argVars := ArgsWithoutContext(method)
	// Inline поля обрабатываются в renderer.ArgsFieldsWithoutContext()
	return argVars
}

// ResultsWithoutError возвращает результаты без последнего error, если он есть.
func ResultsWithoutError(method *parser.Method) []*parser.Variable {

	if IsErrorLast(method.Results) {
		return method.Results[:len(method.Results)-1]
	}
	return method.Results
}

// ResultFieldsWithoutError возвращает результаты без error, обрабатывая inline поля.
// Примечание: Полная обработка inline полей реализована в renderer.inline_fields.go
// Эта функция используется только в контексте, где inline поля не требуются.
func ResultFieldsWithoutError(method *parser.Method) []*parser.Variable {

	resultVars := ResultsWithoutError(method)
	// Inline поля обрабатываются в renderer.ResultFieldsWithoutError()
	return resultVars
}

// RequestStructName возвращает имя структуры request для метода.
func RequestStructName(contractName string, methodName string) string {

	return "request" + contractName + methodName
}

// ResponseStructName возвращает имя структуры response для метода.
func ResponseStructName(contractName string, methodName string) string {

	return "response" + contractName + methodName
}

// RemoveSkippedFields удаляет поля из списка, которые указаны в skipFields.
func RemoveSkippedFields(vars []*parser.Variable, skipFields []string) []*parser.Variable {

	var result []*parser.Variable
	for _, v := range vars {
		if !slices.Contains(skipFields, v.Name) {
			result = append(result, v)
		}
	}
	return result
}

// Arguments возвращает аргументы, которые должны быть в теле запроса.
// Эта функция должна использоваться только в контексте renderer, где есть доступ к аннотациям.
// Для полной реализации нужен доступ к method.Annotations, поэтому эта функция оставлена как заглушка.
func Arguments(method *parser.Method) []*parser.Variable {

	// Эта функция не может быть полностью реализована без доступа к аннотациям метода
	// Полная реализация находится в renderer.arguments()
	return ArgsWithoutContext(method)
}

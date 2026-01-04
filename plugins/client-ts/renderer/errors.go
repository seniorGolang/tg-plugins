// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strconv"
	"strings"

	"tgp/plugins/client-ts/tsg"
	"tgp/shared"
)

// errorInfo структура для хранения информации об ошибке
type errorInfo struct {
	code     int    // HTTP статус код (0 если не указан)
	codeText string // Текстовое описание статус кода
	pkgPath  string // Путь пакета
	typeName string // Имя типа ошибки
}

// collectMethodErrors собирает информацию об ошибках метода
// Возвращает map с ключом "pkgPath:typeName" и значением errorInfo
func (r *ClientRenderer) collectMethodErrors(method *shared.Method, contract *shared.Contract) map[string]errorInfo {

	errorsMap := make(map[string]errorInfo)

	// 1. Ищем ошибки из аннотаций (@tg 401=package:TypeName)
	for key, value := range method.Annotations {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Пытаемся преобразовать ключ в число (HTTP статус код)
		if code, err := strconv.Atoi(key); err == nil {
			// Проверяем, что это валидный HTTP статус код ошибки (4xx, 5xx)
			if code >= 400 && code < 600 {
				if value != "" && value != "skip" {
					// Парсим значение формата "package:TypeName"
					if tokens := strings.Split(value, ":"); len(tokens) == 2 {
						pkgPath := tokens[0]
						typeName := tokens[1]
						key := fmt.Sprintf("%s:%s", pkgPath, typeName)

						codeText := r.getHTTPStatusText(code)
						errorsMap[key] = errorInfo{
							code:     code,
							codeText: codeText,
							pkgPath:  pkgPath,
							typeName: typeName,
						}
					}
				}
			}
		}
	}

	// 2. Используем ошибки из shared.Method.Errors (уже проанализированные)
	for _, errInfo := range method.Errors {
		key := fmt.Sprintf("%s:%s", errInfo.PkgPath, errInfo.TypeName)
		if _, exists := errorsMap[key]; !exists {
			errorsMap[key] = errorInfo{
				code:     errInfo.HTTPCode,
				codeText: errInfo.HTTPCodeText,
				pkgPath:  errInfo.PkgPath,
				typeName: errInfo.TypeName,
			}
		}
	}

	return errorsMap
}

// renderErrorType генерирует TypeScript интерфейс для типа ошибки
func (r *ClientRenderer) renderErrorType(errInfo errorInfo) *tsg.Statement {

	stmt := tsg.NewStatement()

	// Ищем структуру ошибки в project.Types
	var structType *shared.Type
	typeID := fmt.Sprintf("%s:%s", errInfo.pkgPath, errInfo.typeName)
	if typ, ok := r.project.Types[typeID]; ok && typ.Kind == shared.TypeKindStruct {
		structType = typ
	}

	// Если не нашли по полному пути, пробуем найти по имени типа
	if structType == nil {
		for _, typ := range r.project.Types {
			if typ.Kind == shared.TypeKindStruct && typ.TypeName == errInfo.typeName {
				structType = typ
				break
			}
		}
	}

	// Формируем имя типа для TypeScript
	// Извлекаем последнюю часть пути пакета
	pkgParts := strings.Split(errInfo.pkgPath, "/")
	pkgName := pkgParts[len(pkgParts)-1]
	typeName := fmt.Sprintf("%s%s", pkgName, errInfo.typeName)

	// Генерируем интерфейс
	stmt.Comment(fmt.Sprintf("Error type: %s.%s", pkgName, errInfo.typeName))
	if errInfo.code != 0 {
		stmt.Comment(fmt.Sprintf("HTTP status code: %d (%s)", errInfo.code, errInfo.codeText))
	}

	stmt.Export().Interface(typeName, func(grp *tsg.Group) {
		// Добавляем базовое поле message (из метода Error())
		grp.Add(tsg.NewStatement().Id("message").Colon().Id("string").Semicolon())

		// Добавляем поле code (из метода Code())
		if errInfo.code != 0 {
			grp.Add(tsg.NewStatement().Id("code").Colon().Lit(errInfo.code).Semicolon())
		} else {
			grp.Add(tsg.NewStatement().Id("code").Colon().Id("number").Semicolon())
		}

		// Добавляем поля из структуры ошибки
		if structType != nil && structType.StructFields != nil {
			for _, field := range structType.StructFields {
				fieldName, inline := r.jsonName(field)
				if fieldName == "-" || inline {
					continue
				}

				// Пропускаем поля message и code, если они уже есть
				if fieldName == "message" || fieldName == "code" {
					continue
				}

				// Определяем тип поля
				fieldVar := &shared.Variable{
					Name:             field.Name,
					TypeID:            field.TypeID,
					NumberOfPointers:  field.NumberOfPointers,
					IsSlice:           field.IsSlice,
					ArrayLen:           field.ArrayLen,
					MapKeyID:           field.MapKeyID,
					MapValueID:         field.MapValueID,
				}
				// Используем PkgPath из структуры, если он есть
				pkgPath := errInfo.pkgPath
				if structType != nil && structType.ImportPkgPath != "" {
					pkgPath = structType.ImportPkgPath
				}
				// Для полей ошибок используем isArgument=false, так как это возвращаемые значения
				fieldTags := parseTagsFromDocs(field.Docs)
				typeStr := r.walkVariable(field.Name, pkgPath, fieldVar, fieldTags, false).typeLink()

				fieldStmt := tsg.NewStatement()
				fieldStmt.Id(fieldName)

				// Проверяем, является ли поле optional (pointer или omitempty)
				isOptional := false
				if field.NumberOfPointers > 0 {
					isOptional = true
				} else {
					// Проверяем omitempty в JSON тегах
					if tagValues, ok := field.Tags["json"]; ok {
						for _, val := range tagValues {
							if val == "omitempty" {
								isOptional = true
								break
							}
						}
					}
				}

				if isOptional {
					fieldStmt.Optional()
				}

				fieldStmt.Colon()
				fieldStmt.Add(tsg.TypeFromString(typeStr))
				fieldStmt.Semicolon()
				grp.Add(fieldStmt)
			}
		}
	})

	return stmt
}

// renderErrorUnionType генерирует union тип для возможных ошибок метода
func (r *ClientRenderer) renderErrorUnionType(methodName string, errorsMap map[string]errorInfo) *tsg.Statement {

	stmt := tsg.NewStatement()

	// Формируем union тип из всех возможных ошибок
	errorTypes := make([]string, 0, len(errorsMap))
	for _, errInfo := range errorsMap {
		pkgParts := strings.Split(errInfo.pkgPath, "/")
		pkgName := pkgParts[len(pkgParts)-1]
		typeName := fmt.Sprintf("%s%s", pkgName, errInfo.typeName)
		errorTypes = append(errorTypes, typeName)
	}

	// Если есть ошибки, создаём union тип
	if len(errorTypes) > 0 {
		// Сортируем для консистентности
		for i := 0; i < len(errorTypes)-1; i++ {
			for j := i + 1; j < len(errorTypes); j++ {
				if errorTypes[i] > errorTypes[j] {
					errorTypes[i], errorTypes[j] = errorTypes[j], errorTypes[i]
				}
			}
		}

		unionTypeName := fmt.Sprintf("%sError", methodName)
		stmt.Comment(fmt.Sprintf("Union type for possible errors of %s method", methodName))
		stmt.Export().TypeAlias(unionTypeName)

		// Создаём union тип
		unionTypes := make([]*tsg.Statement, len(errorTypes))
		for i, errorType := range errorTypes {
			unionTypes[i] = tsg.NewStatement().Id(errorType)
		}
		stmt.Union(unionTypes...)

		stmt.Semicolon()
	}

	return stmt
}

// getHTTPStatusText возвращает текстовое описание HTTP статус кода
func (r *ClientRenderer) getHTTPStatusText(code int) string {

	// Используем те же тексты, что и в swagger-utils.go
	statusTexts := map[int]string{
		400: "Bad Request",
		401: "Unauthorized",
		402: "Payment Required",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		406: "Not Acceptable",
		407: "Proxy Authentication Required",
		408: "Request Timeout",
		409: "Conflict",
		410: "Gone",
		411: "Length Required",
		412: "Precondition Failed",
		413: "Request Entity Too Large",
		414: "Request URI Too Long",
		415: "Unsupported Media Type",
		416: "Requested Range Not Satisfiable",
		417: "Expectation Failed",
		418: "I'm a teapot",
		422: "Unprocessable Entity",
		423: "Locked",
		424: "Failed Dependency",
		425: "Upgrade Required",
		426: "Precondition Required",
		429: "Too Many Requests",
		431: "Request Header Fields Too Large",
		451: "Unavailable For Legal Reasons",
		500: "Internal Server Error",
		501: "Not Implemented",
		502: "Bad Gateway",
		503: "Service Unavailable",
		504: "Gateway Timeout",
		505: "HTTP Version Not Supported",
		506: "Variant Also Negotiates",
		507: "Insufficient Storage",
		508: "Loop Detected",
		510: "Not Extended",
		511: "Network Authentication Required",
	}

	if text, found := statusTexts[code]; found {
		return text
	}
	return fmt.Sprintf("HTTP Error %d", code)
}


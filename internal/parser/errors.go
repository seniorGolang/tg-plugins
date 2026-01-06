// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"tgp/internal/mod"
	"tgp/internal/tags"
)

// analyzeMethodErrors анализирует ошибки методов из аннотаций и имплементаций.
func analyzeMethodErrors(log *slog.Logger, project *Project) error {
	for _, contract := range project.Contracts {
		for _, method := range contract.Methods {
			// 1. Извлекаем ошибки из аннотаций метода
			errorsFromAnnotations := extractErrorsFromAnnotations(method.Annotations)

			// 2. Извлекаем ошибки из имплементаций
			errorsFromImplementations := extractErrorsFromImplementations(log, method, contract, project)

			// Объединяем ошибки
			errorsMap := make(map[string]*ErrorInfo)
			for _, errInfo := range errorsFromAnnotations {
				key := fmt.Sprintf("%s:%s", errInfo.PkgPath, errInfo.TypeName)
				errorsMap[key] = errInfo
			}
			for _, errInfo := range errorsFromImplementations {
				key := fmt.Sprintf("%s:%s", errInfo.PkgPath, errInfo.TypeName)
				if existing, exists := errorsMap[key]; exists {
					if existing.HTTPCode == 0 && errInfo.HTTPCode != 0 {
						existing.HTTPCode = errInfo.HTTPCode
						existing.HTTPCodeText = errInfo.HTTPCodeText
					}
				} else {
					errorsMap[key] = errInfo
				}
			}

			// Преобразуем map в slice
			method.Errors = make([]*ErrorInfo, 0, len(errorsMap))
			for _, errInfo := range errorsMap {
				method.Errors = append(method.Errors, errInfo)
			}
		}
	}

	return nil
}

// extractErrorsFromAnnotations извлекает ошибки из аннотаций метода.
func extractErrorsFromAnnotations(methodTags tags.DocTags) []*ErrorInfo {
	errors := make([]*ErrorInfo, 0)

	for key, value := range methodTags {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(fmt.Sprintf("%v", value))

		code, err := strconv.Atoi(key)
		if err != nil {
			continue
		}

		if code < 400 || code >= 600 {
			continue
		}

		if value == "" || value == "skip" {
			continue
		}

		tokens := strings.Split(value, ":")
		if len(tokens) != 2 {
			continue
		}

		pkgPath := tokens[0]
		typeName := tokens[1]

		typeID := findErrorTypeID(pkgPath, typeName)
		if typeID == "" {
			typeID = fmt.Sprintf("%s:%s", pkgPath, typeName)
		}

		errInfo := &ErrorInfo{
			PkgPath:      pkgPath,
			TypeName:     typeName,
			FullName:     fmt.Sprintf("%s.%s", pkgPath, typeName),
			HTTPCode:     code,
			HTTPCodeText: getHTTPStatusText(code),
			TypeID:       typeID,
		}

		errors = append(errors, errInfo)
	}

	return errors
}

// extractErrorsFromImplementations извлекает ошибки из имплементаций контракта.
func extractErrorsFromImplementations(log *slog.Logger, method *Method, contract *Contract, project *Project) []*ErrorInfo {
	errorsMap := make(map[string]*ErrorInfo)

	for _, impl := range contract.Implementations {
		implMethod, exists := impl.MethodsMap[method.Name]
		if !exists {
			continue
		}

		for _, errorRef := range implMethod.ErrorTypes {
			key := fmt.Sprintf("%s:%s", errorRef.PkgPath, errorRef.TypeName)
			if _, exists := errorsMap[key]; exists {
				continue
			}

			if isErrorType(log, errorRef.PkgPath, errorRef.TypeName) {
				typeID := findErrorTypeID(errorRef.PkgPath, errorRef.TypeName)
				if typeID == "" {
					typeID = fmt.Sprintf("%s:%s", errorRef.PkgPath, errorRef.TypeName)
				}

				errInfo := &ErrorInfo{
					PkgPath:  errorRef.PkgPath,
					TypeName: errorRef.TypeName,
					FullName: errorRef.FullName,
					TypeID:   typeID,
				}

				errorsMap[key] = errInfo
			}
		}
	}

	errors := make([]*ErrorInfo, 0, len(errorsMap))
	for _, errInfo := range errorsMap {
		errors = append(errors, errInfo)
	}

	return errors
}

// isErrorType проверяет, является ли тип типом ошибки (имеет методы Error() и Code()).
func isErrorType(log *slog.Logger, pkgPath, typeName string) bool {
	goProjectPath := mod.GoProjectPath(".")
	if goProjectPath == "" {
		return false
	}

	possiblePaths := []string{
		pkgPath,
		mod.PkgModPath(pkgPath),
		path.Join("./vendor", pkgPath),
		trimLocalPkg(pkgPath),
	}

	for i, pp := range possiblePaths {
		if strings.HasPrefix(pp, "full-tg/") {
			relPath := strings.TrimPrefix(pp, "full-tg/")
			possiblePaths[i] = path.Join("tests", "full", relPath)
		}
	}

	for _, searchPath := range possiblePaths {
		fullPath := path.Join(goProjectPath, searchPath)
		if _, err := os.Stat(fullPath); err != nil {
			continue
		}

		found := false
		_ = filepath.Walk(fullPath, func(filePath string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
				return nil
			}

			fset := token.NewFileSet()
			srcFile, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
			if err != nil {
				return nil
			}

			for _, decl := range srcFile.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if typeSpec.Name.Name != typeName {
						continue
					}
					_, ok = typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					hasErrorMethod := false
					hasCodeMethod := false

					for _, fileDecl := range srcFile.Decls {
						funcDecl, ok := fileDecl.(*ast.FuncDecl)
						if !ok {
							continue
						}
						if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
							continue
						}
						recvType := funcDecl.Recv.List[0].Type
						if !isReceiverForStruct(recvType, typeName) {
							continue
						}

						if funcDecl.Name == nil {
							continue
						}

						if funcDecl.Name.Name == "Error" && funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
							if ident, ok := funcDecl.Type.Results.List[0].Type.(*ast.Ident); ok && ident.Name == "string" {
								hasErrorMethod = true
							}
						}
						if funcDecl.Name.Name == "Code" && funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
							if ident, ok := funcDecl.Type.Results.List[0].Type.(*ast.Ident); ok && ident.Name == "int" {
								hasCodeMethod = true
							}
						}
					}

					if hasErrorMethod && hasCodeMethod {
						found = true
						return filepath.SkipAll
					}
				}
			}

			return nil
		})

		if found {
			return true
		}
	}

	return false
}

// findErrorTypeID находит ID типа ошибки в проекте.
func findErrorTypeID(pkgPath, typeName string) string {
	return fmt.Sprintf("%s:%s", pkgPath, typeName)
}

// getHTTPStatusText возвращает текстовое описание HTTP статус кода.
func getHTTPStatusText(code int) string {
	statusTexts := map[int]string{
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		409: "Conflict",
		422: "Unprocessable Entity",
		429: "Too Many Requests",
		500: "Internal Server Error",
		502: "Bad Gateway",
		503: "Service Unavailable",
		504: "Gateway Timeout",
	}
	if text, ok := statusTexts[code]; ok {
		return text
	}
	return fmt.Sprintf("HTTP %d", code)
}

// trimLocalPkg обрезает локальную часть пути пакета.
func trimLocalPkg(pkg string) string {
	return pkg
}

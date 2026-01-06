// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"strings"
)

// IsBuiltinTypeName проверяет, является ли имя типа встроенным.
func IsBuiltinTypeName(typeName string) bool {
	return isBuiltinTypeName(typeName)
}

// isBuiltinTypeName проверяет, является ли имя типа встроенным.
func isBuiltinTypeName(typeName string) bool {
	builtinTypes := map[string]bool{
		"bool": true, "string": true, "int": true, "int8": true, "int16": true,
		"int32": true, "int64": true, "uint": true, "uint8": true, "uint16": true,
		"uint32": true, "uint64": true, "uintptr": true, "byte": true, "rune": true,
		"float32": true, "float64": true, "complex64": true, "complex128": true,
		"error": true, "any": true,
	}
	return builtinTypes[typeName]
}

// isExcludedType проверяет, является ли тип исключением.
func isExcludedType(typ *Type, project *Project) bool {
	if typ == nil {
		return false
	}

	// Встроенные типы
	if isBuiltinTypeName(typ.TypeName) {
		return true
	}

	// Явные исключения
	if isExplicitlyExcludedType(typ) {
		return true
	}

	// Проверяем, реализует ли тип json.Marshaler
	if project != nil && typ.ImportPkgPath != "" && typ.TypeName != "" {
		if containsString(typ.ImplementsInterfaces, "encoding/json.Marshaler") {
			return true
		}
	}

	return false
}

// isExplicitlyExcludedType проверяет явные исключения для известных типов.
func isExplicitlyExcludedType(typ *Type) bool {
	// time.Time
	if typ.ImportPkgPath == "time" && typ.TypeName == "Time" {
		return true
	}
	if typ.ImportPkgPath == "" && typ.TypeName == "Time" {
		return true
	}

	// time.Duration
	if typ.ImportPkgPath == "time" && typ.TypeName == "Duration" {
		return true
	}

	// UUID типы
	if strings.HasSuffix(typ.TypeName, "UUID") || typ.TypeName == "UUID" {
		if typ.ImportPkgPath == "" {
			return true
		}
		uuidPackages := []string{
			"github.com/google/uuid",
			"github.com/satori/go.uuid",
			"gopkg.in/guregu/null.v4",
		}
		for _, pkg := range uuidPackages {
			if strings.HasPrefix(typ.ImportPkgPath, pkg) && typ.TypeName != "Time" {
				return true
			}
		}
		if strings.Contains(typ.ImportPkgPath, "uuid") && typ.TypeName != "Time" {
			return true
		}
	}

	// Decimal типы
	if strings.HasSuffix(typ.TypeName, "Decimal") {
		return true
	}

	// big.Int, big.Float, big.Rat
	if typ.ImportPkgPath == "math/big" {
		if typ.TypeName == "Int" || typ.TypeName == "Float" || typ.TypeName == "Rat" {
			return true
		}
	}

	// sql.Null типы
	if typ.ImportPkgPath == "database/sql" {
		if strings.HasPrefix(typ.TypeName, "Null") {
			return true
		}
	}

	// guregu/null типы
	if strings.Contains(typ.ImportPkgPath, "guregu/null") {
		return true
	}

	return false
}

// containsString проверяет, содержится ли строка в слайсе.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package core

import (
	"strings"
)

// GetSerializationFormat возвращает формат сериализации типа для использования в генераторах (Swagger, OpenAPI и т.д.).
// Возвращает openAPIType (string, number, boolean, object) и format (date-time, uuid, int64 и т.д.).
func GetSerializationFormat(typ *Type, project *Project) (openAPIType string, format string) {
	if typ == nil {
		return "string", ""
	}

	// Для алиасов используем базовый тип
	if typ.Kind == TypeKindAlias && typ.AliasOf != "" {
		if baseType, exists := project.Types[typ.AliasOf]; exists {
			return GetSerializationFormat(baseType, project)
		}
		// Если базовый тип не найден, пытаемся определить по typeID
		openAPIType, format := getFormatForExcludedTypeID(typ.AliasOf)
		if openAPIType != "" {
			return openAPIType, format
		}
	}

	// Проверка явных исключений (time.Time, uuid.UUID и т.д.)
	if openAPIType, format := getFormatForExcludedType(typ); openAPIType != "" {
		return openAPIType, format
	}

	// Проверка json.Marshaler (для не-структур сериализуется как строка)
	if containsString(typ.ImplementsInterfaces, "encoding/json.Marshaler") {
		// Структуры с json.Marshaler все равно сериализуются как объекты
		if typ.Kind == TypeKindStruct {
			return "object", ""
		}
		// Для не-структур - как строка
		return "string", ""
	}

	// Использование базового типа (UnderlyingTypeID, UnderlyingKind)
	if typ.UnderlyingKind != "" {
		openAPIType, format = getFormatForBasicType(typ.UnderlyingKind)
		if openAPIType != "" {
			return openAPIType, format
		}
	}

	if typ.UnderlyingTypeID != "" {
		openAPIType, format := getFormatForExcludedTypeID(typ.UnderlyingTypeID)
		if openAPIType != "" {
			return openAPIType, format
		}
	}

	// Определение формата на основе Kind
	return getFormatForTypeKind(typ.Kind)
}

// GetSerializationFormatForTypeID возвращает формат сериализации для типа по его ID.
func GetSerializationFormatForTypeID(typeID string, project *Project) (openAPIType string, format string) {
	if typeID == "" {
		return "string", ""
	}

	// Поиск типа в Project.Types
	if typ, exists := project.Types[typeID]; exists {
		return GetSerializationFormat(typ, project)
	}

	// Если тип не найден, проверяем исключения через typeID
	return getFormatForExcludedTypeID(typeID)
}

// getFormatForExcludedType возвращает формат для исключенного типа.
func getFormatForExcludedType(typ *Type) (openAPIType string, format string) {
	if typ == nil {
		return "", ""
	}

	// time.Time
	if typ.ImportPkgPath == "time" && typ.TypeName == "Time" {
		return "string", "date-time"
	}
	if typ.ImportPkgPath == "" && typ.TypeName == "Time" {
		return "string", "date-time"
	}

	// time.Duration
	if typ.ImportPkgPath == "time" && typ.TypeName == "Duration" {
		return "number", "int64"
	}

	// UUID типы
	if strings.HasSuffix(typ.TypeName, "UUID") || typ.TypeName == "UUID" {
		if typ.ImportPkgPath == "" {
			return "string", "uuid"
		}
		uuidPackages := []string{
			"github.com/google/uuid",
			"github.com/satori/go.uuid",
			"gopkg.in/guregu/null.v4",
		}
		for _, pkg := range uuidPackages {
			if strings.HasPrefix(typ.ImportPkgPath, pkg) && typ.TypeName != "Time" {
				return "string", "uuid"
			}
		}
		if strings.Contains(typ.ImportPkgPath, "uuid") && typ.TypeName != "Time" {
			return "string", "uuid"
		}
	}

	// Decimal типы
	if strings.HasSuffix(typ.TypeName, "Decimal") {
		return "string", ""
	}

	// big.Int, big.Float, big.Rat
	if typ.ImportPkgPath == "math/big" {
		if typ.TypeName == "Int" || typ.TypeName == "Float" || typ.TypeName == "Rat" {
			return "string", ""
		}
	}

	// sql.Null типы
	if typ.ImportPkgPath == "database/sql" {
		if strings.HasPrefix(typ.TypeName, "Null") {
			return "string", ""
		}
	}

	// guregu/null типы
	if strings.Contains(typ.ImportPkgPath, "guregu/null") {
		return "string", ""
	}

	return "", ""
}

// getFormatForExcludedTypeID возвращает формат для исключенного типа по его ID.
func getFormatForExcludedTypeID(typeID string) (openAPIType string, format string) {
	if typeID == "" {
		return "string", ""
	}

	// Разбиваем typeID на части (package:type)
	parts := strings.SplitN(typeID, ":", 2)
	if len(parts) != 2 {
		// Если это встроенный тип
		if isBuiltinTypeName(typeID) {
			return getFormatForBasicType(TypeKind(typeID))
		}
		return "string", ""
	}

	importPkgPath := parts[0]
	typeName := parts[1]

	// time.Time
	if importPkgPath == "time" && typeName == "Time" {
		return "string", "date-time"
	}

	// time.Duration
	if importPkgPath == "time" && typeName == "Duration" {
		return "number", "int64"
	}

	// UUID типы
	if strings.HasSuffix(typeName, "UUID") || typeName == "UUID" {
		if importPkgPath == "" {
			return "string", "uuid"
		}
		uuidPackages := []string{
			"github.com/google/uuid",
			"github.com/satori/go.uuid",
			"gopkg.in/guregu/null.v4",
		}
		for _, pkg := range uuidPackages {
			if strings.HasPrefix(importPkgPath, pkg) && typeName != "Time" {
				return "string", "uuid"
			}
		}
		if strings.Contains(importPkgPath, "uuid") && typeName != "Time" {
			return "string", "uuid"
		}
	}

	// Decimal типы
	if strings.HasSuffix(typeName, "Decimal") {
		return "string", ""
	}

	// big.Int, big.Float, big.Rat
	if importPkgPath == "math/big" {
		if typeName == "Int" || typeName == "Float" || typeName == "Rat" {
			return "string", ""
		}
	}

	// sql.Null типы
	if importPkgPath == "database/sql" {
		if strings.HasPrefix(typeName, "Null") {
			return "string", ""
		}
	}

	// guregu/null типы
	if strings.Contains(importPkgPath, "guregu/null") {
		return "string", ""
	}

	// Если это встроенный тип
	if isBuiltinTypeName(typeName) {
		return getFormatForBasicType(TypeKind(typeName))
	}

	return "string", ""
}

// getFormatForBasicType возвращает формат для базового типа на основе Kind.
func getFormatForBasicType(kind TypeKind) (openAPIType string, format string) {
	switch kind {
	case TypeKindString:
		return "string", ""
	case TypeKindInt, TypeKindInt8, TypeKindInt16, TypeKindInt32, TypeKindInt64:
		if kind == TypeKindInt64 {
			return "number", "int64"
		}
		return "integer", string(kind)
	case TypeKindUint, TypeKindUint8, TypeKindUint16, TypeKindUint32, TypeKindUint64:
		if kind == TypeKindUint64 {
			return "number", "uint64"
		}
		return "integer", string(kind)
	case TypeKindFloat32, TypeKindFloat64:
		if kind == TypeKindFloat64 {
			return "number", "float64"
		}
		return "number", string(kind)
	case TypeKindBool:
		return "boolean", ""
	case TypeKindByte:
		return "integer", "byte"
	case TypeKindRune:
		return "integer", "rune"
	case TypeKindError:
		return "string", ""
	case TypeKindAny:
		return "object", ""
	default:
		return "string", ""
	}
}

// getFormatForTypeKind возвращает формат для типа на основе его Kind.
func getFormatForTypeKind(kind TypeKind) (openAPIType string, format string) {
	switch kind {
	case TypeKindString:
		return "string", ""
	case TypeKindInt, TypeKindInt8, TypeKindInt16, TypeKindInt32, TypeKindInt64:
		if kind == TypeKindInt64 {
			return "number", "int64"
		}
		return "integer", string(kind)
	case TypeKindUint, TypeKindUint8, TypeKindUint16, TypeKindUint32, TypeKindUint64:
		if kind == TypeKindUint64 {
			return "number", "uint64"
		}
		return "integer", string(kind)
	case TypeKindFloat32, TypeKindFloat64:
		if kind == TypeKindFloat64 {
			return "number", "float64"
		}
		return "number", string(kind)
	case TypeKindBool:
		return "boolean", ""
	case TypeKindByte:
		return "integer", "byte"
	case TypeKindRune:
		return "integer", "rune"
	case TypeKindError:
		return "string", ""
	case TypeKindAny:
		return "object", ""
	case TypeKindStruct:
		return "object", ""
	case TypeKindArray, TypeKindMap:
		return "object", ""
	case TypeKindInterface:
		return "object", ""
	case TypeKindFunction:
		return "string", ""
	case TypeKindChan:
		return "string", ""
	case TypeKindAlias:
		// Алиасы должны обрабатываться через AliasOf
		return "string", ""
	default:
		return "string", ""
	}
}

// IsExplicitlyExcludedType проверяет, является ли тип явным исключением (time.Time, uuid.UUID и т.д.).
func IsExplicitlyExcludedType(typ *Type) bool {
	if typ == nil {
		return false
	}
	return isExplicitlyExcludedType(typ)
}

// IsExcludedTypeID проверяет, является ли тип исключением по его ID.
func IsExcludedTypeID(typeID string, project *Project) bool {
	if typeID == "" {
		return false
	}

	// Проверяем встроенные типы
	if isBuiltinTypeName(typeID) {
		return true
	}

	// Проверяем в Project.Types
	if typ, exists := project.Types[typeID]; exists {
		return isExcludedType(typ, project)
	}

	// Проверяем по typeID напрямую
	parts := strings.SplitN(typeID, ":", 2)
	if len(parts) != 2 {
		return false
	}

	importPkgPath := parts[0]
	typeName := parts[1]

	// time.Time
	if importPkgPath == "time" && typeName == "Time" {
		return true
	}

	// time.Duration
	if importPkgPath == "time" && typeName == "Duration" {
		return true
	}

	// UUID типы
	if strings.HasSuffix(typeName, "UUID") || typeName == "UUID" {
		if importPkgPath == "" {
			return true
		}
		uuidPackages := []string{
			"github.com/google/uuid",
			"github.com/satori/go.uuid",
			"gopkg.in/guregu/null.v4",
		}
		for _, pkg := range uuidPackages {
			if strings.HasPrefix(importPkgPath, pkg) && typeName != "Time" {
				return true
			}
		}
		if strings.Contains(importPkgPath, "uuid") && typeName != "Time" {
			return true
		}
	}

	// Decimal типы
	if strings.HasSuffix(typeName, "Decimal") {
		return true
	}

	// big.Int, big.Float, big.Rat
	if importPkgPath == "math/big" {
		if typeName == "Int" || typeName == "Float" || typeName == "Rat" {
			return true
		}
	}

	// sql.Null типы
	if importPkgPath == "database/sql" {
		if strings.HasPrefix(typeName, "Null") {
			return true
		}
	}

	// guregu/null типы
	if strings.Contains(importPkgPath, "guregu/null") {
		return true
	}

	return false
}

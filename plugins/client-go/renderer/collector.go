// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strings"

	"tgp/shared"
)

// CollectTypeIDsForExchange собирает все typeID типов, используемых в exchange структурах контракта.
// Возвращает set typeID для всех типов, которые используются в контракте.
// При генерации будем использовать project.Types напрямую, проверяя для каждого типа,
// нужно ли его генерировать локально (если из текущего проекта) или использовать через импорт.
func (r *ClientRenderer) CollectTypeIDsForExchange(contract *shared.Contract) map[string]bool {

	collectedTypeIDs := make(map[string]bool)
	processedTypes := make(map[string]bool)

	// Собираем typeID из всех методов контракта
	for _, method := range contract.Methods {
		// Собираем типы из аргументов
		for _, arg := range r.argsWithoutContext(method) {
			r.collectTypeIDRecursive(arg.TypeID, collectedTypeIDs, processedTypes)
			if arg.MapKeyID != "" {
				r.collectTypeIDRecursive(arg.MapKeyID, collectedTypeIDs, processedTypes)
			}
			if arg.MapValueID != "" {
				r.collectTypeIDRecursive(arg.MapValueID, collectedTypeIDs, processedTypes)
			}
		}

		// Собираем типы из результатов
		for _, result := range r.resultsWithoutError(method) {
			r.collectTypeIDRecursive(result.TypeID, collectedTypeIDs, processedTypes)
			if result.MapKeyID != "" {
				r.collectTypeIDRecursive(result.MapKeyID, collectedTypeIDs, processedTypes)
			}
			if result.MapValueID != "" {
				r.collectTypeIDRecursive(result.MapValueID, collectedTypeIDs, processedTypes)
			}
		}

		// Собираем типы ошибок из аннотаций методов
		// Ошибки могут быть указаны через @tg 400=, @tg 401= и т.д., а также через defaultError
		for _, errInfo := range method.Errors {
			if errInfo.TypeID != "" {
				r.collectTypeIDRecursive(errInfo.TypeID, collectedTypeIDs, processedTypes)
			}
		}

		// Собираем тип ошибки по умолчанию из аннотации defaultError
		if defaultError, ok := method.Annotations["defaultError"]; ok && defaultError != "" && defaultError != "skip" {
			// Парсим формат "pkgPath:TypeName"
			if tokens := strings.Split(defaultError, ":"); len(tokens) == 2 {
				pkgPath := tokens[0]
				typeName := tokens[1]
				typeID := fmt.Sprintf("%s:%s", pkgPath, typeName)
				r.collectTypeIDRecursive(typeID, collectedTypeIDs, processedTypes)
			}
		}
	}

	return collectedTypeIDs
}

// collectTypeIDRecursive рекурсивно собирает все typeID типов, используемых в контракте.
// Core уже собрал все типы рекурсивно в project.Types, нам нужно только собрать список typeID.
// При генерации будем использовать project.Types напрямую, проверяя для каждого типа,
// нужно ли его генерировать локально (если из текущего проекта) или использовать через импорт.
func (r *ClientRenderer) collectTypeIDRecursive(typeID string, collectedTypeIDs map[string]bool, processedTypes map[string]bool) {

	// Пропускаем уже обработанные типы
	if processedTypes[typeID] {
		return
	}
	processedTypes[typeID] = true

	// Пропускаем встроенные типы
	if r.isBuiltinType(typeID) {
		return
	}

	// Проверяем исключаемые типы
	if r.isExcludedTypeID(typeID) {
		return
	}

	// Получаем тип из project.Types (Core уже собрал все типы рекурсивно)
	typ, ok := r.project.Types[typeID]
	if !ok {
		return
	}

	// Проверяем исключаемые типы
	if r.isExplicitlyExcludedType(typ) {
		return
	}

	// Добавляем typeID в список (независимо от того, из текущего проекта или внешний)
	// При генерации будем проверять, нужно ли генерировать локально
	collectedTypeIDs[typeID] = true

	// Рекурсивно обходим зависимые типы (используя уже собранные данные Core)
	switch typ.Kind {
	case shared.TypeKindArray:
		if typ.ArrayOfID != "" {
			r.collectTypeIDRecursive(typ.ArrayOfID, collectedTypeIDs, processedTypes)
		}
	case shared.TypeKindMap:
		if typ.MapKeyID != "" {
			r.collectTypeIDRecursive(typ.MapKeyID, collectedTypeIDs, processedTypes)
		}
		if typ.MapValueID != "" {
			r.collectTypeIDRecursive(typ.MapValueID, collectedTypeIDs, processedTypes)
		}
	case shared.TypeKindAlias:
		if typ.AliasOf != "" {
			r.collectTypeIDRecursive(typ.AliasOf, collectedTypeIDs, processedTypes)
		}
		if typ.UnderlyingTypeID != "" {
			r.collectTypeIDRecursive(typ.UnderlyingTypeID, collectedTypeIDs, processedTypes)
		}
	case shared.TypeKindStruct:
		for _, field := range typ.StructFields {
			if field.TypeID != "" {
				r.collectTypeIDRecursive(field.TypeID, collectedTypeIDs, processedTypes)
			}
			if field.MapKeyID != "" {
				r.collectTypeIDRecursive(field.MapKeyID, collectedTypeIDs, processedTypes)
			}
			if field.MapValueID != "" {
				r.collectTypeIDRecursive(field.MapValueID, collectedTypeIDs, processedTypes)
			}
		}
	case shared.TypeKindInterface:
		for _, embedded := range typ.EmbeddedInterfaces {
			if embedded.TypeID != "" {
				r.collectTypeIDRecursive(embedded.TypeID, collectedTypeIDs, processedTypes)
			}
		}
		for _, method := range typ.InterfaceMethods {
			for _, arg := range method.Args {
				if arg.TypeID != "" {
					r.collectTypeIDRecursive(arg.TypeID, collectedTypeIDs, processedTypes)
				}
				if arg.MapKeyID != "" {
					r.collectTypeIDRecursive(arg.MapKeyID, collectedTypeIDs, processedTypes)
				}
				if arg.MapValueID != "" {
					r.collectTypeIDRecursive(arg.MapValueID, collectedTypeIDs, processedTypes)
				}
			}
			for _, result := range method.Results {
				if result.TypeID != "" {
					r.collectTypeIDRecursive(result.TypeID, collectedTypeIDs, processedTypes)
				}
				if result.MapKeyID != "" {
					r.collectTypeIDRecursive(result.MapKeyID, collectedTypeIDs, processedTypes)
				}
				if result.MapValueID != "" {
					r.collectTypeIDRecursive(result.MapValueID, collectedTypeIDs, processedTypes)
				}
			}
		}
	}
}

// isBuiltinType реализован в types.go

// isExcludedTypeID проверяет, является ли тип исключением по его ID.
func (r *ClientRenderer) isExcludedTypeID(typeID string) bool {
	if typeID == "" {
		return false
	}

	// Проверяем встроенные типы
	if r.isBuiltinType(typeID) {
		return true
	}

	// Проверяем в Project.Types
	if typ, exists := r.project.Types[typeID]; exists {
		return r.isExcludedType(typ)
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

// isExcludedType проверяет, является ли тип исключением.
func (r *ClientRenderer) isExcludedType(typ *shared.Type) bool {
	if typ == nil {
		return false
	}

	// Встроенные типы
	if r.isBuiltinType(typ.TypeName) {
		return true
	}

	// Явные исключения
	if r.isExplicitlyExcludedType(typ) {
		return true
	}

	// Проверяем, реализует ли тип json.Marshaler
	if typ.ImportPkgPath != "" && typ.TypeName != "" {
		for _, iface := range typ.ImplementsInterfaces {
			if iface == "encoding/json.Marshaler" {
				return true
			}
		}
	}

	return false
}

// isExplicitlyExcludedType проверяет явные исключения для известных типов.
func (r *ClientRenderer) isExplicitlyExcludedType(typ *shared.Type) bool {
	if typ == nil {
		return false
	}

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


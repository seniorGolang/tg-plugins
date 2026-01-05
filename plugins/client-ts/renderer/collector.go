// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strings"

	"tgp/core"
)

// CollectTypeIDsForExchange собирает все typeID типов, используемых в exchange структурах контракта.
// Возвращает set typeID для всех типов, которые используются в контракте.
// ВАЖНО: для TS нужно собирать ВСЕ типы, включая внешние либы, так как они должны быть сгенерированы.
func (r *ClientRenderer) CollectTypeIDsForExchange(contract *core.Contract) map[string]bool {

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
// ВАЖНО: для TS собираем ВСЕ типы, включая внешние либы, так как они должны быть сгенерированы.
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

	// ВАЖНО: для TS НЕ пропускаем исключаемые типы на этапе сбора,
	// так как они должны быть обработаны при генерации (например, time.Time -> Date)
	// Проверяем только явные исключения, которые не нужно обрабатывать вообще

	// Получаем тип из project.Types (Core уже собрал все типы рекурсивно)
	typ, ok := r.project.Types[typeID]
	if !ok {
		// Тип не найден - возможно это встроенный тип или исключаемый тип
		// Но для TS мы все равно можем его использовать (например, time.Time -> Date)
		return
	}

	// ВАЖНО: для TS добавляем ВСЕ типы, включая внешние либы
	// При генерации будем проверять, нужно ли генерировать локально или использовать через namespace
	collectedTypeIDs[typeID] = true

	// Рекурсивно обходим зависимые типы (используя уже собранные данные Core)
	switch typ.Kind {
	case core.TypeKindArray:
		if typ.ArrayOfID != "" {
			r.collectTypeIDRecursive(typ.ArrayOfID, collectedTypeIDs, processedTypes)
		}
	case core.TypeKindMap:
		if typ.MapKeyID != "" {
			r.collectTypeIDRecursive(typ.MapKeyID, collectedTypeIDs, processedTypes)
		}
		if typ.MapValueID != "" {
			r.collectTypeIDRecursive(typ.MapValueID, collectedTypeIDs, processedTypes)
		}
	case core.TypeKindAlias:
		if typ.AliasOf != "" {
			r.collectTypeIDRecursive(typ.AliasOf, collectedTypeIDs, processedTypes)
		}
		if typ.UnderlyingTypeID != "" {
			r.collectTypeIDRecursive(typ.UnderlyingTypeID, collectedTypeIDs, processedTypes)
		}
	case core.TypeKindStruct:
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
	case core.TypeKindInterface:
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

// isBuiltinType проверяет, является ли тип встроенным типом Go.
func (r *ClientRenderer) isBuiltinType(typeID string) bool {
	builtinTypes := map[string]bool{
		"string":  true,
		"int":     true,
		"int8":    true,
		"int16":   true,
		"int32":   true,
		"int64":   true,
		"uint":    true,
		"uint8":   true,
		"uint16":  true,
		"uint32":  true,
		"uint64":  true,
		"float32": true,
		"float64": true,
		"bool":    true,
		"byte":    true,
		"rune":    true,
		"error":   true,
		"any":     true,
	}
	return builtinTypes[typeID]
}

// isExplicitlyExcludedType проверяет явные исключения для известных типов.
// ВАЖНО: для TS эти типы не исключаются полностью, а преобразуются (например, time.Time -> Date).
func (r *ClientRenderer) isExplicitlyExcludedType(typ *core.Type) bool {
	if typ == nil {
		return false
	}

	// time.Time - преобразуется в Date
	if typ.ImportPkgPath == "time" && typ.TypeName == "Time" {
		return false // НЕ исключаем, преобразуем
	}
	if typ.ImportPkgPath == "" && typ.TypeName == "Time" {
		return false // НЕ исключаем, преобразуем
	}

	// time.Duration - преобразуется в number
	if typ.ImportPkgPath == "time" && typ.TypeName == "Duration" {
		return false // НЕ исключаем, преобразуем
	}

	// UUID типы - преобразуются в string
	if strings.HasSuffix(typ.TypeName, "UUID") || typ.TypeName == "UUID" {
		return false // НЕ исключаем, преобразуем
	}

	// Decimal типы - преобразуются в number
	if strings.HasSuffix(typ.TypeName, "Decimal") {
		return false // НЕ исключаем, преобразуем
	}

	// big.Int, big.Float, big.Rat - преобразуются в number
	if typ.ImportPkgPath == "math/big" {
		if typ.TypeName == "Int" || typ.TypeName == "Float" || typ.TypeName == "Rat" {
			return false // НЕ исключаем, преобразуем
		}
	}

	// sql.Null типы - преобразуются в соответствующие типы
	if typ.ImportPkgPath == "database/sql" {
		if strings.HasPrefix(typ.TypeName, "Null") {
			return false // НЕ исключаем, преобразуем
		}
	}

	// guregu/null типы - преобразуются в соответствующие типы
	if strings.Contains(typ.ImportPkgPath, "guregu/null") {
		return false // НЕ исключаем, преобразуем
	}

	return false
}

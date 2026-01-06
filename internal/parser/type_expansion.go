// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
)

// expandTypesRecursively рекурсивно разбирает все типы из контрактов.
// Использует подход из gopls/internal/typesinternal/element.go:ForEachElement
func expandTypesRecursively(log *slog.Logger, project *Project) error {
	seenTypes := &typeutil.Map{}
	msets := &typeutil.MethodSetCache{}

	// Собираем все типы из контрактов
	for _, contract := range project.Contracts {
		for _, method := range contract.Methods {
			// Обрабатываем аргументы
			for _, arg := range method.Args {
				if err := collectTypeFromID(log, arg.TypeID, project, seenTypes, msets); err != nil {
					return err
				}
			}

			// Обрабатываем результаты
			for _, result := range method.Results {
				if err := collectTypeFromID(log, result.TypeID, project, seenTypes, msets); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// collectTypeFromID получает types.Type по typeID и рекурсивно обходит все его зависимости.
func collectTypeFromID(log *slog.Logger, typeID string, project *Project, seenTypes *typeutil.Map, msets *typeutil.MethodSetCache) error {
	// Получаем тип из project.Types
	typ, exists := project.Types[typeID]
	if !exists {
		// Тип не найден - пытаемся загрузить его из пакета
		if err := ensureTypeLoaded(log, typeID, project); err != nil {
			log.Debug("Failed to load type %s: %v", typeID, err)
			return nil
		}
		typ, exists = project.Types[typeID]
		if !exists {
			return nil
		}
	}

	// Проверяем, является ли тип исключением
	if isExcludedType(typ, project) {
		return nil
	}

	// Если это алиас, обрабатываем базовый тип рекурсивно
	if typ.Kind == TypeKindAlias && typ.AliasOf != "" {
		// Обрабатываем базовый тип рекурсивно
		// collectTypeFromID загрузит базовый тип через ensureTypeLoaded, если его еще нет
		if err := collectTypeFromID(log, typ.AliasOf, project, seenTypes, msets); err != nil {
			log.Debug("Failed to collect base type for alias", "aliasOf", typ.AliasOf, "typeID", typeID, "error", err)
			// Продолжаем обработку, даже если базовый тип не загружен
		}
		// Продолжаем обработку текущего типа для обработки его полей (если есть)
		// Но если это просто алиас без полей, можно вернуться
		if len(typ.StructFields) == 0 {
			// Это простой алиас - базовый тип уже обработан (или будет обработан), можно вернуться
			return nil
		}
	}

	// Получаем types.Type из пакета для рекурсивного обхода
	if typ.ImportPkgPath == "" || typ.TypeName == "" {
		return nil
	}

	pkgInfo, err := getPackageInfo(log, typ.ImportPkgPath)
	if err != nil || pkgInfo == nil || pkgInfo.Types == nil {
		return nil
	}

	obj := pkgInfo.Types.Scope().Lookup(typ.TypeName)
	if obj == nil {
		return nil
	}

	typeNameObj, ok := obj.(*types.TypeName)
	if !ok {
		return nil
	}

	// Рекурсивно обходим все типы, начиная с этого типа
	forEachReachableType(log, typeNameObj.Type(), project, seenTypes, msets)

	// Также обрабатываем базовый тип алиаса, если он еще не обработан
	if typ.Kind == TypeKindAlias && typ.AliasOf != "" {
		// Убеждаемся, что базовый тип обработан рекурсивно
		if baseType, exists := project.Types[typ.AliasOf]; exists {
			// Обрабатываем базовый тип рекурсивно через forEachReachableType
			if baseType.ImportPkgPath != "" && baseType.TypeName != "" {
				basePkgInfo, err := getPackageInfo(log, baseType.ImportPkgPath)
				if err == nil && basePkgInfo != nil && basePkgInfo.Types != nil {
					baseObj := basePkgInfo.Types.Scope().Lookup(baseType.TypeName)
					if baseObj != nil {
						if baseTypeNameObj, ok := baseObj.(*types.TypeName); ok {
							forEachReachableType(log, baseTypeNameObj.Type(), project, seenTypes, msets)
						}
					}
				}
			}
		}
	}

	return nil
}

// forEachReachableType рекурсивно обходит все типы, достижимые из данного типа.
// Основано на gopls/internal/typesinternal/element.go:ForEachElement
func forEachReachableType(log *slog.Logger, t types.Type, project *Project, seenTypes *typeutil.Map, msets *typeutil.MethodSetCache) {
	var visit func(t types.Type, skip bool)
	visit = func(t types.Type, skip bool) {
		if !skip {
			// Проверяем, не обрабатывали ли мы уже этот тип
			if seen, _ := seenTypes.At(t).(bool); seen {
				return
			}
			seenTypes.Set(t, true)

			// Сохраняем тип в project.Types
			saveTypeFromGoTypes(log, t, project)
		}

		// Рекурсия по сигнатурам всех методов
		tmset := msets.MethodSet(t)
		for method := range tmset.Methods() {
			sig := method.Type().(*types.Signature)
			visit(sig.Params(), true)
			visit(sig.Results(), true)
		}

		// Рекурсивный обход в зависимости от вида типа
		switch t := t.(type) {
		case *types.Alias:
			// Обрабатываем базовый тип алиаса
			visit(types.Unalias(t), skip)
			// Также сохраняем сам алиас, если он еще не сохранен
			if !skip {
				saveTypeFromGoTypes(log, t, project)
			}

		case *types.Basic:
			// nop

		case *types.Interface:
			// nop

		case *types.Pointer:
			visit(t.Elem(), false)

		case *types.Slice:
			visit(t.Elem(), false)

		case *types.Chan:
			visit(t.Elem(), false)

		case *types.Map:
			visit(t.Key(), false)
			visit(t.Elem(), false)

		case *types.Signature:
			// Пропускаем сигнатуры с Recv - они обрабатываются через method set
			if t.Recv() != nil {
				return
			}
			visit(t.Params(), true)
			visit(t.Results(), true)

		case *types.Named:
			// Добавляем указатель на именованный тип
			ptrType := types.NewPointer(t)
			visit(ptrType, false)
			visit(t.Underlying(), true)

		case *types.Array:
			visit(t.Elem(), false)

		case *types.Struct:
			for i, n := 0, t.NumFields(); i < n; i++ {
				visit(t.Field(i).Type(), false)
			}

		case *types.Tuple:
			for i, n := 0, t.Len(); i < n; i++ {
				visit(t.At(i).Type(), false)
			}

		case *types.TypeParam, *types.Union:
			log.Debug("Skipping generic type", "type", fmt.Sprintf("%T", t))

		default:
			log.Debug("Unknown type in forEachReachableType", "type", fmt.Sprintf("%T", t))
		}
	}
	visit(t, false)
}

// saveTypeFromGoTypes сохраняет тип из go/types в project.Types.
func saveTypeFromGoTypes(log *slog.Logger, t types.Type, project *Project) {
	typeID := generateTypeIDFromGoTypes(t)
	if typeID == "" {
		return
	}

	// Проверяем, не является ли это встроенным типом
	if basic, ok := t.(*types.Basic); ok {
		if isBuiltinTypeName(basic.Name()) {
			return
		}
	}

	// Проверяем, не сохранен ли уже тип
	if _, exists := project.Types[typeID]; exists {
		return
	}

	// Получаем информацию о пакете
	var importPkgPath string
	var typeName string

	switch t := t.(type) {
	case *types.Named:
		if t.Obj() != nil {
			typeName = t.Obj().Name()
			if t.Obj().Pkg() != nil {
				importPkgPath = t.Obj().Pkg().Path()
			}
		}
	case *types.Alias:
		if t.Obj() != nil {
			typeName = t.Obj().Name()
			if t.Obj().Pkg() != nil {
				importPkgPath = t.Obj().Pkg().Path()
			}
		}
	}

	if importPkgPath == "" || typeName == "" {
		return
	}

	pkgInfo, err := getPackageInfo(log, importPkgPath)
	if err != nil || pkgInfo == nil {
		return
	}

	// Конвертируем тип через convertTypeFromGoTypes
	// Создаем processingSet для защиты от рекурсии
	processingSet := make(map[string]bool)
	coreType := convertTypeFromGoTypes(log, t, importPkgPath, pkgInfo.Imports, project, processingSet)
	if coreType == nil {
		return
	}

	// Сохраняем тип
	// ВАЖНО: для базовых типов алиасов не применяем isExcludedType,
	// так как они нужны для правильной обработки алиасов
	// Проверяем, является ли этот тип базовым для какого-то алиаса
	isBaseTypeOfAlias := false
	for _, typ := range project.Types {
		if typ.Kind == TypeKindAlias && typ.AliasOf == typeID {
			isBaseTypeOfAlias = true
			break
		}
	}

	if !isExcludedType(coreType, project) || isBaseTypeOfAlias {
		project.Types[typeID] = coreType
	}
}

// generateTypeIDFromGoTypes генерирует typeID для types.Type.
func generateTypeIDFromGoTypes(t types.Type) string {
	switch t := t.(type) {
	case *types.Basic:
		return t.Name()

	case *types.Named:
		if t.Obj() != nil {
			typeName := t.Obj().Name()
			if t.Obj().Pkg() != nil {
				importPkgPath := t.Obj().Pkg().Path()
				return fmt.Sprintf("%s:%s", importPkgPath, typeName)
			}
			return typeName
		}
		return ""

	case *types.Alias:
		if t.Obj() != nil {
			typeName := t.Obj().Name()
			if t.Obj().Pkg() != nil {
				importPkgPath := t.Obj().Pkg().Path()
				return fmt.Sprintf("%s:%s", importPkgPath, typeName)
			}
			return typeName
		}
		return ""

	default:
		return ""
	}
}

// ensureTypeLoaded загружает тип из пакета, если его еще нет в project.Types.
func ensureTypeLoaded(log *slog.Logger, typeID string, project *Project) error {
	parts := splitTypeID(typeID)
	if len(parts) != 2 {
		return nil
	}

	importPkgPath := parts[0]
	typeName := parts[1]

	if isBuiltinTypeName(typeName) {
		return nil
	}

	pkgInfo, err := getPackageInfo(log, importPkgPath)
	if err != nil || pkgInfo == nil || pkgInfo.Types == nil {
		return fmt.Errorf("package %s not found: %w", importPkgPath, err)
	}

	obj := pkgInfo.Types.Scope().Lookup(typeName)
	if obj == nil {
		return fmt.Errorf("type %s not found in package %s", typeName, importPkgPath)
	}

	typeNameObj, ok := obj.(*types.TypeName)
	if !ok {
		return fmt.Errorf("object %s is not a type name", typeName)
	}

	// Конвертируем тип
	// Создаем processingSet для защиты от рекурсии
	processingSet := make(map[string]bool)
	coreType := convertTypeFromGoTypes(log, typeNameObj.Type(), importPkgPath, pkgInfo.Imports, project, processingSet)
	if coreType == nil {
		return fmt.Errorf("failed to convert type %s", typeID)
	}

	// Сохраняем тип
	// ВАЖНО: для базовых типов алиасов не применяем isExcludedType,
	// так как они нужны для правильной обработки алиасов
	// Проверяем, является ли этот тип базовым для какого-то алиаса
	isBaseTypeOfAlias := false
	for _, typ := range project.Types {
		if typ.Kind == TypeKindAlias && typ.AliasOf == typeID {
			isBaseTypeOfAlias = true
			break
		}
	}

	if !isExcludedType(coreType, project) || isBaseTypeOfAlias {
		project.Types[typeID] = coreType
	}

	// Если это алиас, убеждаемся, что базовый тип также обработан рекурсивно
	if coreType.Kind == TypeKindAlias && coreType.AliasOf != "" {
		// Базовый тип уже должен быть обработан в convertTypeFromGoTypes,
		// но убеждаемся, что он есть в project.Types
		if _, exists := project.Types[coreType.AliasOf]; !exists {
			// Пытаемся загрузить базовый тип
			if err := ensureTypeLoaded(log, coreType.AliasOf, project); err != nil {
				log.Debug("Failed to load base type %s: %v", coreType.AliasOf, err)
			}
		}
	}

	return nil
}

// splitTypeID разбивает typeID на части.
func splitTypeID(typeID string) []string {
	// Простая реализация - ищем последний ":"
	idx := strings.LastIndex(typeID, ":")
	if idx == -1 {
		return []string{typeID}
	}
	return []string{typeID[:idx], typeID[idx+1:]}
}

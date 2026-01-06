// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/types"
	"log/slog"
)

// convertTypeFromGoTypes конвертирует types.Type в Type.
// Основано на подходе из gopls: работаем напрямую с go/types.
// processingTypes используется для защиты от рекурсии при циклических зависимостях.
func convertTypeFromGoTypes(log *slog.Logger, typ types.Type, pkgPath string, imports map[string]string, project *Project, processingTypes ...map[string]bool) *Type {
	if typ == nil {
		return nil
	}

	// Создаем или используем существующий set обрабатываемых типов
	var processingSet map[string]bool
	if len(processingTypes) > 0 && processingTypes[0] != nil {
		processingSet = processingTypes[0]
	} else {
		processingSet = make(map[string]bool)
	}

	// Генерируем typeID для проверки рекурсии
	typeID := generateTypeIDFromGoTypes(typ)
	if typeID == "" {
		if basic, ok := typ.(*types.Basic); ok {
			typeID = basic.Name()
		}
	}

	// Если тип уже существует в project.Types, возвращаем его
	if typeID != "" && !isBuiltinTypeName(typeID) {
		if existingType, exists := project.Types[typeID]; exists {
			return existingType
		}
	}

	// Проверяем, не обрабатываем ли мы уже этот тип (рекурсивный случай)
	if typeID != "" && !isBuiltinTypeName(typeID) {
		if processingSet[typeID] {
			// Уже обрабатываем этот тип - возвращаем уже созданный или создаем минимальный
			if existingType, exists := project.Types[typeID]; exists {
				return existingType
			}
			// Создаем минимальный тип для рекурсивного случая
			coreType := &Type{}
			if named, ok := typ.(*types.Named); ok {
				if named.Obj() != nil {
					coreType.TypeName = named.Obj().Name()
					if named.Obj().Pkg() != nil {
						coreType.ImportPkgPath = named.Obj().Pkg().Path()
						coreType.PkgName = named.Obj().Pkg().Name()
					}
				}
			} else if alias, ok := typ.(*types.Alias); ok {
				if alias.Obj() != nil {
					coreType.TypeName = alias.Obj().Name()
					if alias.Obj().Pkg() != nil {
						coreType.ImportPkgPath = alias.Obj().Pkg().Path()
						coreType.PkgName = alias.Obj().Pkg().Name()
					}
				}
			}
			project.Types[typeID] = coreType
			return coreType
		}

		// Помечаем тип как обрабатываемый
		processingSet[typeID] = true
	}

	coreType := &Type{}

	// Для именованных типов создаем базовую структуру сразу
	if typeID != "" && !isBuiltinTypeName(typeID) {
		if named, ok := typ.(*types.Named); ok {
			if named.Obj() != nil {
				coreType.TypeName = named.Obj().Name()
				if named.Obj().Pkg() != nil {
					coreType.ImportPkgPath = named.Obj().Pkg().Path()
					coreType.PkgName = named.Obj().Pkg().Name()
				}
			}
		} else if alias, ok := typ.(*types.Alias); ok {
			if alias.Obj() != nil {
				coreType.TypeName = alias.Obj().Name()
				if alias.Obj().Pkg() != nil {
					coreType.ImportPkgPath = alias.Obj().Pkg().Path()
					coreType.PkgName = alias.Obj().Pkg().Name()
				}
			}
		}

		// Создаем тип в project.Types ДО обработки полей - это позволяет правильно обработать рекурсивные типы
		project.Types[typeID] = coreType
	}

	switch t := typ.(type) {
	case *types.Basic:
		coreType.Kind = convertBasicKind(t.Kind())
		coreType.TypeName = t.Name()

	case *types.Named:
		if t.Obj() != nil {
			coreType.TypeName = t.Obj().Name()
			if t.Obj().Pkg() != nil {
				coreType.ImportPkgPath = t.Obj().Pkg().Path()
				coreType.PkgName = t.Obj().Pkg().Name()
				for alias, path := range imports {
					if path == coreType.ImportPkgPath {
						coreType.ImportAlias = alias
						break
					}
				}
			}
		}

		underlying := t.Underlying()

		// Для именованных типов, которые являются массивами/слайсами (например, UUID = [16]byte),
		// сохраняем информацию о типе, а не о массиве
		// Это позволяет правильно использовать тип в генерации кода
		if _, isArray := underlying.(*types.Array); isArray {
			// Именованный тип, который является массивом - сохраняем как именованный тип
			// Kind будет установлен на основе underlying, но ArrayOfID не заполняем
			coreType.Kind = TypeKindArray
			// Для массива сохраняем длину и тип элемента
			if arrayType, ok := underlying.(*types.Array); ok {
				coreType.ArrayLen = int(arrayType.Len())
				if arrayType.Elem() != nil {
					elemInfo := convertTypeFromGoTypesToInfo(log, arrayType.Elem(), coreType.ImportPkgPath, imports, project)
					coreType.ArrayOfID = elemInfo.TypeID
				}
			}
		} else if _, isSlice := underlying.(*types.Slice); isSlice {
			// Именованный тип, который является слайсом - сохраняем как именованный тип
			coreType.Kind = TypeKindArray
			coreType.IsSlice = true
			if sliceType, ok := underlying.(*types.Slice); ok {
				if sliceType.Elem() != nil {
					elemInfo := convertTypeFromGoTypesToInfo(log, sliceType.Elem(), coreType.ImportPkgPath, imports, project)
					coreType.ArrayOfID = elemInfo.TypeID
				}
			}
		} else {
			// Для остальных типов используем стандартную логику
			coreType.Kind = resolveKindFromUnderlying(underlying)

			if structType, ok := underlying.(*types.Struct); ok {
				coreType.Kind = TypeKindStruct
				fillStructFields(log, structType, coreType.ImportPkgPath, imports, project, coreType, processingSet)
			}
		}

	case *types.Alias:
		if t.Obj() != nil {
			coreType.TypeName = t.Obj().Name()
			if t.Obj().Pkg() != nil {
				coreType.ImportPkgPath = t.Obj().Pkg().Path()
				coreType.PkgName = t.Obj().Pkg().Name()
				for alias, path := range imports {
					if path == coreType.ImportPkgPath {
						coreType.ImportAlias = alias
						break
					}
				}
			}
		}

		underlying := types.Unalias(t)
		// Устанавливаем Kind на основе underlying типа, но сохраняем информацию об алиасе
		coreType.Kind = resolveKindFromUnderlying(underlying)
		// Если underlying - именованный тип, это алиас
		if _, ok := underlying.(*types.Named); ok {
			coreType.Kind = TypeKindAlias
		}

		if named, ok := underlying.(*types.Named); ok {
			if named.Obj() != nil {
				coreType.AliasOf = fmt.Sprintf("%s:%s", named.Obj().Pkg().Path(), named.Obj().Name())
				// Сохраняем базовый тип, если его еще нет
				baseTypeID := coreType.AliasOf
				if _, exists := project.Types[baseTypeID]; !exists {
					basePkgPath := named.Obj().Pkg().Path()
					basePkgInfo, err := getPackageInfo(log, basePkgPath)
					switch {
					case err != nil:
						log.Debug("Failed to get package info for base type %s: %v", baseTypeID, err)
						// Базовый тип будет обработан позже в expandTypesRecursively через ensureTypeLoaded
					case basePkgInfo != nil:
						// Обрабатываем базовый тип рекурсивно
						// Важно: используем тот же processingSet для защиты от рекурсии
						baseCoreType := convertTypeFromGoTypes(log, named, basePkgPath, basePkgInfo.Imports, project, processingSet)
						if baseCoreType != nil {
							// ВАЖНО: сохраняем базовый тип БЕЗ проверки isExcludedType,
							// так как он нужен для правильной обработки алиаса
							// Проверка isExcludedType применяется только при сохранении через saveTypeFromGoTypes
							project.Types[baseTypeID] = baseCoreType
							log.Debug("Saved base type for alias", "baseTypeID", baseTypeID)
							// Поля базового типа уже обработаны в convertTypeFromGoTypes через fillStructFields
							// Базовый тип будет обработан рекурсивно в expandTypesRecursively через collectTypeFromID
						} else {
							log.Debug("Failed to convert base type", "baseTypeID", baseTypeID)
						}
					default:
						log.Debug("Package info is nil for base type", "baseTypeID", baseTypeID)
						// Базовый тип будет обработан позже в expandTypesRecursively через ensureTypeLoaded
					}
				}
				// Базовый тип уже существует - убеждаемся, что он обработан рекурсивно
				// Это произойдет в expandTypesRecursively через collectTypeFromID
			}
		} else if basic, ok := underlying.(*types.Basic); ok {
			coreType.UnderlyingKind = convertBasicKind(basic.Kind())
			coreType.UnderlyingTypeID = basic.Name()
		} else if structType, ok := underlying.(*types.Struct); ok {
			// Алиас на структуру - заполняем поля
			coreType.Kind = TypeKindStruct
			fillStructFields(log, structType, coreType.ImportPkgPath, imports, project, coreType, processingSet)
		}

	case *types.Struct:
		coreType.Kind = TypeKindStruct
		fillStructFields(log, t, pkgPath, imports, project, coreType, processingSet)

	case *types.Interface:
		coreType.Kind = TypeKindInterface

	default:
		log.Debug("Unknown type in convertTypeFromGoTypes", "type", fmt.Sprintf("%T", typ))
		return nil
	}

	detectInterfaces(log, typ, coreType, project)

	return coreType
}

// convertBasicKind конвертирует types.BasicKind в TypeKind.
func convertBasicKind(kind types.BasicKind) TypeKind {
	switch kind {
	case types.String:
		return TypeKindString
	case types.Int:
		return TypeKindInt
	case types.Int8:
		return TypeKindInt8
	case types.Int16:
		return TypeKindInt16
	case types.Int32:
		return TypeKindInt32
	case types.Int64:
		return TypeKindInt64
	case types.Uint:
		return TypeKindUint
	case types.Uint8:
		return TypeKindUint8
	case types.Uint16:
		return TypeKindUint16
	case types.Uint32:
		return TypeKindUint32
	case types.Uint64:
		return TypeKindUint64
	case types.Float32:
		return TypeKindFloat32
	case types.Float64:
		return TypeKindFloat64
	case types.Bool:
		return TypeKindBool
	case types.UntypedNil:
		return TypeKindAny
	default:
		// Handle Byte and Rune which are aliases for Uint8 and Int32
		if kind == types.Byte {
			return TypeKindByte
		}
		if kind == types.Rune {
			return TypeKindRune
		}
		return TypeKindAny
	}
}

// resolveKindFromUnderlying определяет Kind из underlying типа.
func resolveKindFromUnderlying(underlying types.Type) TypeKind {
	switch t := underlying.(type) {
	case *types.Basic:
		return convertBasicKind(t.Kind())
	case *types.Struct:
		return TypeKindStruct
	case *types.Interface:
		return TypeKindInterface
	case *types.Slice, *types.Array:
		return TypeKindArray
	case *types.Map:
		return TypeKindMap
	case *types.Chan:
		return TypeKindChan
	case *types.Signature:
		return TypeKindFunction
	case *types.Named, *types.Alias:
		return TypeKindAlias
	default:
		return TypeKindAny
	}
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log/slog"
	"strconv"
	"strings"

	"tgp/internal/tags"
)

// convertMethod преобразует ast.FuncType в Method.
func convertMethod(log *slog.Logger, methodName string, funcType *ast.FuncType, docs []string, contractID, pkgPath string, imports map[string]string, typeInfo *types.Info, project *Project) *Method {
	methodAnnotations := tags.ParseTags(docs)
	method := &Method{
		Name:        methodName,
		ContractID:  contractID,
		Docs:        removeAnnotationsFromDocs(docs),
		Annotations: methodAnnotations,
		Args:        make([]*Variable, 0),
		Results:     make([]*Variable, 0),
	}

	// Преобразуем аргументы
	if funcType.Params != nil {
		for _, param := range funcType.Params.List {
			convertedTypeInfo := convertTypeFromAST(log, param.Type, pkgPath, imports, project)
			if convertedTypeInfo.TypeID == "" && convertedTypeInfo.MapKeyID == "" {
				log.Warn("Failed to convert type for parameter in method", "method", methodName)
				continue
			}

			paramDocs := extractComments(param.Doc, param.Comment)
			paramAnnotations := tags.ParseTags(paramDocs)

			// Обрабатываем имена параметров
			if len(param.Names) > 0 {
				for _, name := range param.Names {
					method.Args = append(method.Args, &Variable{
						Name:             name.Name,
						TypeID:           convertedTypeInfo.TypeID,
						NumberOfPointers: convertedTypeInfo.NumberOfPointers,
						IsSlice:          convertedTypeInfo.IsSlice,
						ArrayLen:         convertedTypeInfo.ArrayLen,
						IsEllipsis:       convertedTypeInfo.IsEllipsis,
						ElementPointers:  convertedTypeInfo.ElementPointers,
						MapKeyID:         convertedTypeInfo.MapKeyID,
						MapValueID:       convertedTypeInfo.MapValueID,
						MapKeyPointers:   convertedTypeInfo.MapKeyPointers,
						Docs:             removeAnnotationsFromDocs(paramDocs),
						Annotations:      paramAnnotations,
					})
				}
			} else {
				// Анонимный параметр
				method.Args = append(method.Args, &Variable{
					Name:             "",
					TypeID:           convertedTypeInfo.TypeID,
					NumberOfPointers: convertedTypeInfo.NumberOfPointers,
					IsSlice:          convertedTypeInfo.IsSlice,
					ArrayLen:         convertedTypeInfo.ArrayLen,
					IsEllipsis:       convertedTypeInfo.IsEllipsis,
					ElementPointers:  convertedTypeInfo.ElementPointers,
					MapKeyID:         convertedTypeInfo.MapKeyID,
					MapValueID:       convertedTypeInfo.MapValueID,
					MapKeyPointers:   convertedTypeInfo.MapKeyPointers,
					Docs:             removeAnnotationsFromDocs(paramDocs),
					Annotations:      paramAnnotations,
				})
			}
		}
	}

	// Преобразуем результаты
	if funcType.Results != nil {
		for _, result := range funcType.Results.List {
			resultTypeInfo := convertTypeFromAST(log, result.Type, pkgPath, imports, project)
			// Если TypeID пустой, пытаемся определить тип напрямую из AST
			if resultTypeInfo.TypeID == "" && resultTypeInfo.MapKeyID == "" {
				// Пытаемся получить тип через go/types
				pkgInfo, err := getPackageInfo(log, pkgPath)
				if err == nil && pkgInfo != nil && pkgInfo.TypeInfo != nil {
					typ := pkgInfo.TypeInfo.TypeOf(result.Type)
					if typ != nil {
						typeInfo := convertTypeFromGoTypesToInfo(log, typ, pkgPath, imports, project)
						resultTypeInfo = typeInfo
					}
				}
				// Если все еще пусто, пытаемся определить из AST
				if resultTypeInfo.TypeID == "" && resultTypeInfo.MapKeyID == "" {
					if ident, ok := result.Type.(*ast.Ident); ok {
						resultTypeInfo.TypeID = ident.Name
					} else {
						log.Warn("Failed to convert type for result in method", "method", methodName, "resultType", result.Type)
						continue
					}
				}
			}

			resultDocs := extractComments(result.Doc, result.Comment)
			resultAnnotations := tags.ParseTags(resultDocs)

			// Обрабатываем имена результатов
			if len(result.Names) > 0 {
				for _, name := range result.Names {
					method.Results = append(method.Results, &Variable{
						Name:             name.Name,
						TypeID:           resultTypeInfo.TypeID,
						NumberOfPointers: resultTypeInfo.NumberOfPointers,
						IsSlice:          resultTypeInfo.IsSlice,
						ArrayLen:         resultTypeInfo.ArrayLen,
						IsEllipsis:       resultTypeInfo.IsEllipsis,
						ElementPointers:  resultTypeInfo.ElementPointers,
						MapKeyID:         resultTypeInfo.MapKeyID,
						MapValueID:       resultTypeInfo.MapValueID,
						MapKeyPointers:   resultTypeInfo.MapKeyPointers,
						Docs:             removeAnnotationsFromDocs(resultDocs),
						Annotations:      resultAnnotations,
					})
				}
			} else {
				// Анонимный результат
				method.Results = append(method.Results, &Variable{
					Name:             "",
					TypeID:           resultTypeInfo.TypeID,
					NumberOfPointers: resultTypeInfo.NumberOfPointers,
					IsSlice:          resultTypeInfo.IsSlice,
					ArrayLen:         resultTypeInfo.ArrayLen,
					IsEllipsis:       resultTypeInfo.IsEllipsis,
					ElementPointers:  resultTypeInfo.ElementPointers,
					MapKeyID:         resultTypeInfo.MapKeyID,
					MapValueID:       resultTypeInfo.MapValueID,
					MapKeyPointers:   resultTypeInfo.MapKeyPointers,
					Docs:             removeAnnotationsFromDocs(resultDocs),
					Annotations:      resultAnnotations,
				})
			}
		}
	}

	// Извлечение информации о handler
	method.Handler = extractHandlerInfo(method.Annotations)

	return method
}

// extractHandlerInfo извлекает информацию о handler из аннотаций.
func extractHandlerInfo(methodTags tags.DocTags) *HandlerInfo {
	handlerValue, exists := methodTags["handler"]
	if !exists {
		return nil
	}

	// Формат: package:HandlerName
	tokens := fmt.Sprintf("%v", handlerValue)
	parts := strings.Split(tokens, ":")
	if len(parts) != 2 {
		return nil
	}

	return &HandlerInfo{
		PkgPath: parts[0],
		Name:    parts[1],
	}
}

// TypeConversionInfo содержит информацию о преобразованном типе.
type TypeConversionInfo struct {
	TypeID           string
	NumberOfPointers int
	IsSlice          bool
	ArrayLen         int
	IsEllipsis       bool
	ElementPointers  int // Для элементов массивов/слайсов и значений map
	MapKeyID         string
	MapValueID       string
	MapKeyPointers   int
}

// convertTypeFromAST преобразует AST тип в TypeConversionInfo.
// Основано на подходе из gopls: используем go/types напрямую.
func convertTypeFromAST(log *slog.Logger, astType ast.Expr, pkgPath string, imports map[string]string, project *Project) TypeConversionInfo {
	info := TypeConversionInfo{}

	if astType == nil {
		return info
	}

	// Получаем информацию о пакете
	pkgInfo, err := getPackageInfo(log, pkgPath)
	if err != nil || pkgInfo == nil || pkgInfo.TypeInfo == nil {
		log.Warn("Package not found or has no TypeInfo", "pkgPath", pkgPath)
		return info
	}

	// Проверяем ellipsis ДО обработки через go/types
	if ellipsis, ok := astType.(*ast.Ellipsis); ok {
		info.IsEllipsis = true
		info.IsSlice = true
		if ellipsis.Elt != nil {
			eltTyp := pkgInfo.TypeInfo.TypeOf(ellipsis.Elt)
			if eltTyp != nil {
				eltInfo := convertTypeFromGoTypesToInfo(log, eltTyp, pkgPath, imports, project)
				info.TypeID = eltInfo.TypeID
				info.ElementPointers = eltInfo.NumberOfPointers
			} else {
				// Если TypeOf вернул nil, пытаемся обработать напрямую из AST
				if ident, ok := ellipsis.Elt.(*ast.Ident); ok {
					if isBuiltinTypeName(ident.Name) {
						info.TypeID = ident.Name
					} else if pkgInfo.Types != nil {
						// Это может быть тип из текущего пакета
						obj := pkgInfo.Types.Scope().Lookup(ident.Name)
						if obj != nil {
							if typeName, ok := obj.(*types.TypeName); ok {
								typeID := generateTypeIDFromGoTypes(typeName.Type())
								if typeID != "" {
									info.TypeID = typeID
								}
							}
						}
					}
				} else if selExpr, ok := ellipsis.Elt.(*ast.SelectorExpr); ok {
					// Обрабатываем SelectorExpr (например, dto.SomeStruct)
					if x, ok := selExpr.X.(*ast.Ident); ok {
						importAlias := x.Name
						typeName := selExpr.Sel.Name
						importPkgPath, ok := imports[importAlias]
						if ok {
							typeID := fmt.Sprintf("%s:%s", importPkgPath, typeName)
							info.TypeID = typeID
							// Сохраняем тип в project.Types
							if _, exists := project.Types[typeID]; !exists {
								importPkgInfo, err := getPackageInfo(log, importPkgPath)
								if err == nil && importPkgInfo != nil && importPkgInfo.Types != nil {
									obj := importPkgInfo.Types.Scope().Lookup(typeName)
									if obj != nil {
										if typeNameObj, ok := obj.(*types.TypeName); ok {
											processingSet := make(map[string]bool)
											coreType := convertTypeFromGoTypes(log, typeNameObj.Type(), importPkgPath, importPkgInfo.Imports, project, processingSet)
											if coreType != nil {
												project.Types[typeID] = coreType
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return info
	}

	// Сначала проверяем базовые типы напрямую из AST
	if ident, ok := astType.(*ast.Ident); ok {
		// Проверяем, является ли это базовым типом
		if isBuiltinTypeName(ident.Name) {
			info.TypeID = ident.Name
			return info
		}
	}

	// Обрабатываем массивы и слайсы ДО обработки указателей
	// Это важно для правильной обработки базовых типов в массивах
	if arrayType, ok := astType.(*ast.ArrayType); ok {
		info.IsSlice = arrayType.Len == nil // Если Len == nil, это слайс, иначе массив
		if arrayType.Len != nil {
			// Это массив, пытаемся получить длину
			if basicLit, ok := arrayType.Len.(*ast.BasicLit); ok {
				// Парсим длину массива
				if basicLit.Kind == token.INT {
					if arrayLen, err := strconv.Atoi(basicLit.Value); err == nil {
						info.ArrayLen = arrayLen
					}
				}
			}
		}
		if arrayType.Elt != nil {
			// Обрабатываем элемент массива/слайса
			// ВАЖНО: обрабатываем указатели на элементе (например, []*dto.SomeStruct)
			eltASTType := arrayType.Elt
			eltPointers := 0
			// Подсчитываем указатели на элементе из AST
			for {
				if starExpr, ok := eltASTType.(*ast.StarExpr); ok {
					eltPointers++
					eltASTType = starExpr.X
					continue
				}
				break
			}
			// Получаем тип элемента через go/types
			eltTyp := pkgInfo.TypeInfo.TypeOf(arrayType.Elt)
			if eltTyp != nil {
				// Убираем указатели из типа, так как мы уже учли их в eltPointers
				baseEltTyp := eltTyp
				for i := 0; i < eltPointers; i++ {
					if ptr, ok := baseEltTyp.(*types.Pointer); ok {
						baseEltTyp = ptr.Elem()
					} else {
						break
					}
				}
				eltInfo := convertTypeFromGoTypesToInfo(log, baseEltTyp, pkgPath, imports, project)
				info.TypeID = eltInfo.TypeID
				info.ElementPointers = eltPointers
			} else {
				// Если TypeOf вернул nil, пытаемся обработать напрямую из AST
				if ident, ok := eltASTType.(*ast.Ident); ok {
					if isBuiltinTypeName(ident.Name) {
						info.TypeID = ident.Name
						info.ElementPointers = eltPointers
					} else if pkgInfo.Types != nil {
						// Это может быть тип из текущего пакета
						obj := pkgInfo.Types.Scope().Lookup(ident.Name)
						if obj != nil {
							if typeName, ok := obj.(*types.TypeName); ok {
								typeID := generateTypeIDFromGoTypes(typeName.Type())
								if typeID != "" {
									info.TypeID = typeID
									info.ElementPointers = eltPointers
								}
							}
						}
					}
				} else if selExpr, ok := eltASTType.(*ast.SelectorExpr); ok {
					// Обрабатываем SelectorExpr (например, dto.SomeStruct)
					if x, ok := selExpr.X.(*ast.Ident); ok {
						importAlias := x.Name
						typeName := selExpr.Sel.Name
						importPkgPath, ok := imports[importAlias]
						if ok {
							typeID := fmt.Sprintf("%s:%s", importPkgPath, typeName)
							info.TypeID = typeID
							info.ElementPointers = eltPointers
							// Сохраняем тип в project.Types
							if _, exists := project.Types[typeID]; !exists {
								importPkgInfo, err := getPackageInfo(log, importPkgPath)
								if err == nil && importPkgInfo != nil && importPkgInfo.Types != nil {
									obj := importPkgInfo.Types.Scope().Lookup(typeName)
									if obj != nil {
										if typeNameObj, ok := obj.(*types.TypeName); ok {
											processingSet := make(map[string]bool)
											coreType := convertTypeFromGoTypes(log, typeNameObj.Type(), importPkgPath, importPkgInfo.Imports, project, processingSet)
											if coreType != nil {
												project.Types[typeID] = coreType
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return info
	}

	// Обрабатываем мапы ДО обработки указателей
	if mapType, ok := astType.(*ast.MapType); ok {
		if mapType.Key != nil {
			// Обрабатываем указатели на ключе (например, map[*string]int)
			keyASTType := mapType.Key
			keyPointers := 0
			for {
				if starExpr, ok := keyASTType.(*ast.StarExpr); ok {
					keyPointers++
					keyASTType = starExpr.X
					continue
				}
				break
			}
			keyTyp := pkgInfo.TypeInfo.TypeOf(mapType.Key)
			if keyTyp != nil {
				// Убираем указатели из типа
				baseKeyTyp := keyTyp
				for i := 0; i < keyPointers; i++ {
					if ptr, ok := baseKeyTyp.(*types.Pointer); ok {
						baseKeyTyp = ptr.Elem()
					} else {
						break
					}
				}
				keyInfo := convertTypeFromGoTypesToInfo(log, baseKeyTyp, pkgPath, imports, project)
				info.MapKeyID = keyInfo.TypeID
				info.MapKeyPointers = keyPointers
			} else {
				// Если TypeOf вернул nil, пытаемся обработать напрямую из AST
				if ident, ok := mapType.Key.(*ast.Ident); ok {
					if isBuiltinTypeName(ident.Name) {
						info.MapKeyID = ident.Name
					} else if pkgInfo.Types != nil {
						// Это может быть тип из текущего пакета
						obj := pkgInfo.Types.Scope().Lookup(ident.Name)
						if obj != nil {
							if typeName, ok := obj.(*types.TypeName); ok {
								typeID := generateTypeIDFromGoTypes(typeName.Type())
								if typeID != "" {
									info.MapKeyID = typeID
								}
							}
						}
					}
				} else if selExpr, ok := mapType.Key.(*ast.SelectorExpr); ok {
					// Обрабатываем SelectorExpr (например, dto.UserID)
					if x, ok := selExpr.X.(*ast.Ident); ok {
						importAlias := x.Name
						typeName := selExpr.Sel.Name
						importPkgPath, ok := imports[importAlias]
						if ok {
							typeID := fmt.Sprintf("%s:%s", importPkgPath, typeName)
							info.MapKeyID = typeID
							// Сохраняем тип в project.Types
							if _, exists := project.Types[typeID]; !exists {
								importPkgInfo, err := getPackageInfo(log, importPkgPath)
								if err == nil && importPkgInfo != nil && importPkgInfo.Types != nil {
									obj := importPkgInfo.Types.Scope().Lookup(typeName)
									if obj != nil {
										if typeNameObj, ok := obj.(*types.TypeName); ok {
											processingSet := make(map[string]bool)
											coreType := convertTypeFromGoTypes(log, typeNameObj.Type(), importPkgPath, importPkgInfo.Imports, project, processingSet)
											if coreType != nil {
												project.Types[typeID] = coreType
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		if mapType.Value != nil {
			// ВАЖНО: обрабатываем указатели на значении (например, map[string]*dto.SomeStruct)
			valueASTType := mapType.Value
			valuePointers := 0
			// Подсчитываем указатели на значении из AST
			for {
				if starExpr, ok := valueASTType.(*ast.StarExpr); ok {
					valuePointers++
					valueASTType = starExpr.X
					continue
				}
				break
			}
			valueTyp := pkgInfo.TypeInfo.TypeOf(mapType.Value)
			if valueTyp != nil {
				// Убираем указатели из типа, так как мы уже учли их в valuePointers
				baseValueTyp := valueTyp
				for i := 0; i < valuePointers; i++ {
					if ptr, ok := baseValueTyp.(*types.Pointer); ok {
						baseValueTyp = ptr.Elem()
					} else {
						break
					}
				}
				valueInfo := convertTypeFromGoTypesToInfo(log, baseValueTyp, pkgPath, imports, project)
				info.MapValueID = valueInfo.TypeID
				info.ElementPointers = valuePointers
			} else {
				// Если TypeOf вернул nil, пытаемся обработать напрямую из AST
				info.ElementPointers = valuePointers
				if ident, ok := valueASTType.(*ast.Ident); ok {
					if isBuiltinTypeName(ident.Name) {
						info.MapValueID = ident.Name
					}
				} else if selExpr, ok := valueASTType.(*ast.SelectorExpr); ok {
					// Обрабатываем SelectorExpr (например, dto.SomeStruct)
					if x, ok := selExpr.X.(*ast.Ident); ok {
						importAlias := x.Name
						typeName := selExpr.Sel.Name
						importPkgPath, ok := imports[importAlias]
						if ok {
							typeID := fmt.Sprintf("%s:%s", importPkgPath, typeName)
							info.MapValueID = typeID
							// Сохраняем тип в project.Types
							if _, exists := project.Types[typeID]; !exists {
								importPkgInfo, err := getPackageInfo(log, importPkgPath)
								if err == nil && importPkgInfo != nil && importPkgInfo.Types != nil {
									obj := importPkgInfo.Types.Scope().Lookup(typeName)
									if obj != nil {
										if typeNameObj, ok := obj.(*types.TypeName); ok {
											processingSet := make(map[string]bool)
											coreType := convertTypeFromGoTypes(log, typeNameObj.Type(), importPkgPath, importPkgInfo.Imports, project, processingSet)
											if coreType != nil {
												project.Types[typeID] = coreType
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return info
	}

	// Обрабатываем указатели из AST
	baseASTType := astType
	for {
		if starExpr, ok := baseASTType.(*ast.StarExpr); ok {
			info.NumberOfPointers++
			baseASTType = starExpr.X
			continue
		}
		break
	}

	// Обрабатываем *ast.SelectorExpr (например, dto.JSONWebKeySet или *dto.JSONWebKeySet после удаления указателя)
	var typ types.Type
	if selExpr, ok := baseASTType.(*ast.SelectorExpr); ok {
		if x, ok := selExpr.X.(*ast.Ident); ok {
			importAlias := x.Name
			typeName := selExpr.Sel.Name
			// Находим путь пакета по алиасу импорта
			importPkgPath, ok := imports[importAlias]
			if !ok {
				log.Warn("Import alias not found in imports map", "importAlias", importAlias)
				return info
			}
			// Загружаем тип из импортированного пакета
			importPkgInfo, err := getPackageInfo(log, importPkgPath)
			if err != nil || importPkgInfo == nil || importPkgInfo.Types == nil {
				log.Warn("Failed to get package info", "importPkgPath", importPkgPath)
				return info
			}
			obj := importPkgInfo.Types.Scope().Lookup(typeName)
			if obj == nil {
				log.Warn("Type not found in package", "typeName", typeName, "importPkgPath", importPkgPath)
				return info
			}
			typeNameObj, ok := obj.(*types.TypeName)
			if !ok {
				log.Warn("Object in package is not a TypeName", "typeName", typeName, "importPkgPath", importPkgPath)
				return info
			}
			typ = typeNameObj.Type()
			// Генерируем typeID для типа (может быть алиасом)
			// Используем typeNameObj напрямую для правильной обработки алиасов
			typeID := fmt.Sprintf("%s:%s", importPkgPath, typeName)
			if typeID != "" {
				info.TypeID = typeID
				// Сохраняем тип в project.Types
				if _, exists := project.Types[typeID]; !exists {
					// Создаем processingSet для защиты от рекурсии
					processingSet := make(map[string]bool)
					coreType := convertTypeFromGoTypes(log, typ, importPkgPath, importPkgInfo.Imports, project, processingSet)
					if coreType != nil {
						project.Types[typeID] = coreType
						// Если это алиас, базовый тип уже должен быть обработан в convertTypeFromGoTypes
						// Но убеждаемся, что он есть в project.Types
						if alias, ok := typ.(*types.Alias); ok {
							underlying := types.Unalias(alias)
							if named, ok := underlying.(*types.Named); ok {
								baseTypeID := generateTypeIDFromGoTypes(named)
								if baseTypeID != "" && baseTypeID != typeID {
									if _, exists := project.Types[baseTypeID]; !exists {
										// Базовый тип должен был быть обработан в convertTypeFromGoTypes
										// Но если его нет, обрабатываем его
										if named.Obj() != nil && named.Obj().Pkg() != nil {
											basePkgPath := named.Obj().Pkg().Path()
											basePkgInfo, err := getPackageInfo(log, basePkgPath)
											if err == nil && basePkgInfo != nil {
												baseCoreType := convertTypeFromGoTypes(log, named, basePkgPath, basePkgInfo.Imports, project, processingSet)
												if baseCoreType != nil {
													project.Types[baseTypeID] = baseCoreType
												}
											}
										}
									}
								}
							}
						}
					}
				}
				return info
			}
		}
	}

	// Используем go/types для получения типа
	if typ == nil {
		typ = pkgInfo.TypeInfo.TypeOf(astType)
	}
	if typ == nil {
		// Если TypeOf вернул nil, пытаемся обработать напрямую из AST
		if ident, ok := baseASTType.(*ast.Ident); ok {
			// Проверяем базовые типы
			if isBuiltinTypeName(ident.Name) {
				info.TypeID = ident.Name
				return info
			}
			// Это может быть тип из текущего пакета
			if pkgInfo.Types != nil {
				obj := pkgInfo.Types.Scope().Lookup(ident.Name)
				if obj != nil {
					if typeName, ok := obj.(*types.TypeName); ok {
						typ = typeName.Type()
						// Обрабатываем алиасы
						if alias, ok := typ.(*types.Alias); ok {
							typ = types.Unalias(alias)
						}
						// Генерируем typeID
						typeID := generateTypeIDFromGoTypes(typ)
						if typeID != "" {
							info.TypeID = typeID
							// Сохраняем тип в project.Types
							if _, exists := project.Types[typeID]; !exists {
								processingSet := make(map[string]bool)
								coreType := convertTypeFromGoTypes(log, typ, pkgPath, imports, project, processingSet)
								if coreType != nil {
									project.Types[typeID] = coreType
								}
							}
							return info
						}
					}
				}
			}
		}
		if typ == nil {
			log.Warn("Failed to get type from AST", "astType", astType)
			return info
		}
	}

	// Обрабатываем алиасы
	if alias, ok := typ.(*types.Alias); ok {
		typ = types.Unalias(alias)
	}

	// Убираем указатели из typ, так как мы уже учли их в info.NumberOfPointers
	for {
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = ptr.Elem()
			continue
		}
		break
	}

	// Обрабатываем слайсы и массивы
	switch t := typ.(type) {
	case *types.Slice:
		info.IsSlice = true
		if t.Elem() != nil {
			// Находим элемент в AST для правильной обработки указателей на элементе
			if arrayType, ok := baseASTType.(*ast.ArrayType); ok && arrayType.Elt != nil {
				eltTyp := pkgInfo.TypeInfo.TypeOf(arrayType.Elt)
				if eltTyp != nil {
					eltInfo := convertTypeFromGoTypesToInfo(log, eltTyp, pkgPath, imports, project)
					info.TypeID = eltInfo.TypeID
					info.ElementPointers = eltInfo.NumberOfPointers
				}
			} else {
				eltInfo := convertTypeFromGoTypesToInfo(log, t.Elem(), pkgPath, imports, project)
				info.TypeID = eltInfo.TypeID
				info.ElementPointers = eltInfo.NumberOfPointers
			}
		}
		return info

	case *types.Array:
		info.IsSlice = false
		info.ArrayLen = int(t.Len())
		if t.Elem() != nil {
			if arrayType, ok := baseASTType.(*ast.ArrayType); ok && arrayType.Elt != nil {
				eltTyp := pkgInfo.TypeInfo.TypeOf(arrayType.Elt)
				if eltTyp != nil {
					eltInfo := convertTypeFromGoTypesToInfo(log, eltTyp, pkgPath, imports, project)
					info.TypeID = eltInfo.TypeID
					info.ElementPointers = eltInfo.NumberOfPointers
				}
			} else {
				eltInfo := convertTypeFromGoTypesToInfo(log, t.Elem(), pkgPath, imports, project)
				info.TypeID = eltInfo.TypeID
				info.ElementPointers = eltInfo.NumberOfPointers
			}
		}
		return info

	case *types.Map:
		if t.Key() != nil {
			keyInfo := convertTypeFromGoTypesToInfo(log, t.Key(), pkgPath, imports, project)
			info.MapKeyID = keyInfo.TypeID
			info.MapKeyPointers = keyInfo.NumberOfPointers
		}
		if t.Elem() != nil {
			valueInfo := convertTypeFromGoTypesToInfo(log, t.Elem(), pkgPath, imports, project)
			info.MapValueID = valueInfo.TypeID
			info.ElementPointers = valueInfo.NumberOfPointers
		}
		return info
	}

	// Для остальных типов используем convertTypeFromGoTypesToInfo
	typeInfo := convertTypeFromGoTypesToInfo(log, typ, pkgPath, imports, project)
	info.TypeID = typeInfo.TypeID
	// NumberOfPointers уже установлен при обработке указателей из AST
	// Но если тип был получен через go/types, нужно учесть указатели из typeInfo
	if info.NumberOfPointers == 0 {
		info.NumberOfPointers = typeInfo.NumberOfPointers
	}

	return info
}

// convertTypeFromGoTypesToInfo конвертирует types.Type в TypeConversionInfo.
func convertTypeFromGoTypesToInfo(log *slog.Logger, typ types.Type, pkgPath string, imports map[string]string, project *Project) TypeConversionInfo {
	info := TypeConversionInfo{}

	if typ == nil {
		return info
	}

	// Убираем указатели
	for {
		if ptr, ok := typ.(*types.Pointer); ok {
			info.NumberOfPointers++
			typ = ptr.Elem()
			continue
		}
		break
	}

	// Генерируем typeID
	typeID := generateTypeIDFromGoTypes(typ)
	if typeID == "" {
		if basic, ok := typ.(*types.Basic); ok {
			typeID = basic.Name()
		}
	}

	info.TypeID = typeID

	// Сохраняем тип в project.Types, если это именованный тип
	if typeID != "" && !isBuiltinTypeName(typeID) {
		if _, exists := project.Types[typeID]; !exists {
			if named, ok := typ.(*types.Named); ok {
				if named.Obj() != nil && named.Obj().Pkg() != nil {
					importPkgPath := named.Obj().Pkg().Path()
					pkgInfo, err := getPackageInfo(log, importPkgPath)
					if err == nil && pkgInfo != nil {
						processingSet := make(map[string]bool)
						coreType := convertTypeFromGoTypes(log, typ, importPkgPath, pkgInfo.Imports, project, processingSet)
						if coreType != nil {
							project.Types[typeID] = coreType
						}
					}
				}
			} else if alias, ok := typ.(*types.Alias); ok {
				if alias.Obj() != nil && alias.Obj().Pkg() != nil {
					importPkgPath := alias.Obj().Pkg().Path()
					pkgInfo, err := getPackageInfo(log, importPkgPath)
					if err == nil && pkgInfo != nil {
						processingSet := make(map[string]bool)
						coreType := convertTypeFromGoTypes(log, typ, importPkgPath, pkgInfo.Imports, project, processingSet)
						if coreType != nil {
							project.Types[typeID] = coreType
						}
					}
				}
			}
		}
	}

	return info
}

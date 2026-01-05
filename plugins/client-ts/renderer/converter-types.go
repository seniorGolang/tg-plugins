// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strings"

	"tgp/core"
)

// hasMarshaler проверяет, имеет ли тип соответствующий маршалер (Marshaler/Unmarshaler)
// Проверяет только сам тип, без рекурсии по вложенным типам
func (r *ClientRenderer) hasMarshaler(typ *core.Type, isArgument bool) bool {
	if typ == nil {
		return false
	}

	// Проверяем маршалеры самого типа
	// ВАЖНО: интерфейсы хранятся в формате "pkgPath:InterfaceName" (с двоеточием), а не "pkgPath.InterfaceName"
	if len(typ.ImplementsInterfaces) > 0 {
		for _, iface := range typ.ImplementsInterfaces {
			if isArgument {
				// Для аргументов проверяем Marshaler или TextMarshaler (сериализация при отправке)
				if iface == "encoding/json:Marshaler" || iface == "encoding/text:Marshaler" {
					return true
				}
			} else {
				// Для возвращаемых значений проверяем Unmarshaler или TextUnmarshaler (десериализация при получении)
				if iface == "encoding/json:Unmarshaler" || iface == "encoding/text:Unmarshaler" {
					return true
				}
			}
		}
	}

	// Для алиасов проверяем базовый тип (но не рекурсивно по его содержимому)
	if typ.Kind == core.TypeKindAlias && typ.AliasOf != "" {
		if baseType, exists := r.project.Types[typ.AliasOf]; exists {
			return r.hasMarshaler(baseType, isArgument)
		}
	}

	return false
}

// walkVariable конвертирует Go тип в TypeScript определение типа
// isArgument указывает, является ли это аргументом метода (true) или возвращаемым значением (false)
func (r *ClientRenderer) walkVariable(typeName, pkgPath string, variable *core.Variable, varTags map[string]string, isArgument bool) (schema typeDefTs) {
	logger := core.GetLogger()
	logger.Debug(fmt.Sprintf("walkVariable: typeName=%s, pkgPath=%s, variable.TypeID=%s", typeName, pkgPath, variable.TypeID))
	return r.walkVariableWithVisited(typeName, pkgPath, variable, varTags, make(map[string]bool), isArgument)
}

// walkVariableWithVisited конвертирует Go тип в TypeScript определение типа с отслеживанием обрабатываемых типов
// isArgument указывает, является ли это аргументом метода (true) или возвращаемым значением (false)
func (r *ClientRenderer) walkVariableWithVisited(typeName, pkgPath string, variable *core.Variable, varTags map[string]string, processing map[string]bool, isArgument bool) (schema typeDefTs) {

	schema.name = typeName
	schema.properties = make(map[string]typeDefTs)
	if varTags != nil && annotationIsSet(varTags, "nullable") {
		schema.nullable = annotationValue(varTags, "nullable", "") == "true"
	}

	// Обрабатываем массивы и слайсы ПЕРЕД получением типа из project.Types
	// Это важно, потому что для массива []string мы обрабатываем элемент string, а не сам массив
	if variable.IsSlice || variable.ArrayLen > 0 {
		schema.kind = "array"
		schema.typeName = "array"
		schema.nullable = true
		if variable.TypeID != "" {
			itemVar := &core.Variable{
				Name:   "item",
				TypeID: variable.TypeID,
			}
			schema.properties["item"] = r.walkVariableWithVisited("item", pkgPath, itemVar, nil, processing, isArgument)
		}
		return
	}

	// Обрабатываем map ПЕРЕД получением типа из project.Types
	// НО: если TypeID указывает на именованный map тип из текущего проекта, используем его
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		// Проверяем, является ли TypeID именованным map типом из текущего проекта
		if variable.TypeID != "" {
			if typ, ok := r.project.Types[variable.TypeID]; ok {
				if typ.Kind == core.TypeKindMap && typ.TypeName != "" && typ.ImportPkgPath != "" {
					if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
						// Это именованный map тип из текущего проекта - используем ссылку на тип
						schema.kind = "scalar"
						schema.typeName = typ.TypeName
						schema.name = typ.TypeName
						// Сохраняем информацию об импорте
						if typ.PkgName != "" {
							schema.importPkg = typ.PkgName
						} else if typ.ImportAlias != "" {
							schema.importPkg = typ.ImportAlias
						}
						schema.importName = typ.TypeName
						// Сохраняем тип в typeDefTs
						if schema.importPkg != "" && schema.importName != "" {
							typeKey := fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
							// Проверяем, не сохранен ли уже этот тип
							if existingDef, exists := r.typeDefTs[typeKey]; !exists || len(existingDef.properties) == 0 {
								// Сохраняем map определение для генерации type alias
								mapDef := typeDefTs{
									kind:       "map",
									typeName:   "map",
									name:       typ.TypeName,
									importPkg:  schema.importPkg,
									importName: schema.importName,
									properties: map[string]typeDefTs{
										"key":   r.walkVariableWithVisited("key", pkgPath, &core.Variable{TypeID: typ.MapKeyID}, nil, processing, isArgument),
										"value": r.walkVariableWithVisited("value", pkgPath, &core.Variable{TypeID: typ.MapValueID}, nil, processing, isArgument),
									},
								}
								r.typeDefTs[typeKey] = mapDef
							}
						}
						return schema
					}
				}
			}
		}
		// Для неименованных map типов генерируем map напрямую
		schema.kind = "map"
		schema.typeName = "map"
		schema.nullable = true
		keyVar := &core.Variable{TypeID: variable.MapKeyID}
		valueVar := &core.Variable{TypeID: variable.MapValueID}
		schema.properties["key"] = r.walkVariableWithVisited("key", pkgPath, keyVar, nil, processing, isArgument)
		schema.properties["value"] = r.walkVariableWithVisited("value", pkgPath, valueVar, nil, processing, isArgument)
		return
	}

	// Получаем тип из project.Types
	typeID := variable.TypeID
	typ, ok := r.project.Types[typeID]
	// ВАЖНО: если TypeID указывает на именованный map тип из текущего проекта,
	// используем ссылку на тип, даже если MapKeyID и MapValueID пустые
	if ok && typ.Kind == core.TypeKindMap && typ.TypeName != "" && typ.ImportPkgPath != "" {
		if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
			// Это именованный map тип из текущего проекта - используем ссылку на тип
			schema.kind = "scalar"
			schema.typeName = typ.TypeName
			schema.name = typ.TypeName
			// Сохраняем информацию об импорте
			if typ.PkgName != "" {
				schema.importPkg = typ.PkgName
			} else if typ.ImportAlias != "" {
				schema.importPkg = typ.ImportAlias
			}
			schema.importName = typ.TypeName
			// Сохраняем тип в typeDefTs
			if schema.importPkg != "" && schema.importName != "" {
				typeKey := fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
				// Проверяем, не сохранен ли уже этот тип
				if existingDef, exists := r.typeDefTs[typeKey]; !exists || len(existingDef.properties) == 0 {
					// Сохраняем map определение для генерации type alias
					mapKeyID := typ.MapKeyID
					mapValueID := typ.MapValueID
					if mapKeyID == "" {
						mapKeyID = "string"
					}
					if mapValueID == "" {
						mapValueID = "any"
					}
					mapDef := typeDefTs{
						kind:       "map",
						typeName:   "map",
						name:       typ.TypeName,
						importPkg:  schema.importPkg,
						importName: schema.importName,
						properties: map[string]typeDefTs{
							"key":   r.walkVariableWithVisited("key", pkgPath, &core.Variable{TypeID: mapKeyID}, nil, processing, isArgument),
							"value": r.walkVariableWithVisited("value", pkgPath, &core.Variable{TypeID: mapValueID}, nil, processing, isArgument),
						},
					}
					r.typeDefTs[typeKey] = mapDef
				}
			}
			return schema
		}
	}
	if !ok {
		// Тип не найден - возможно это встроенный тип или исключаемый тип (time.Time, UUID и т.п.)
		// Проверяем, является ли это time.Time
		if strings.Contains(typeID, "time") && strings.Contains(typeID, "Time") {
			schema.kind = "scalar"
			schema.typeName = "Date"
			return
		}
		// Проверяем, является ли это UUID
		if strings.Contains(typeID, "UUID") {
			schema.kind = "scalar"
			schema.typeName = "string"
			return
		}
		// Для остальных используем typeIDToTSType
		typeStr := r.typeIDToTSType(typeID)
		schema.kind = "scalar"
		schema.typeName = typeStr
		return
	}

	// ВАЖНО: Проверяем маршалеры ПЕРЕД проверкой исключений
	// Типы с маршалерами должны быть any, независимо от того, являются ли они исключениями
	// (кроме явных исключений, формат которых известен)
	// Проверяем только сам тип, без рекурсии по вложенным типам
	hasCustomMarshaler := r.hasMarshaler(typ, isArgument)
	isExcluded := r.isExplicitlyExcludedType(typ)

	// Если тип имеет кастомный маршалер и НЕ является явным исключением,
	// используем any, так как мы не знаем формат сериализации
	// ВАЖНО: не обрабатываем содержимое типа (поля, элементы и т.д.), просто возвращаем any
	// НО сохраняем тип в typeDefTs как алиас на any, чтобы его можно было использовать в других типах
	if hasCustomMarshaler && !isExcluded {
		schema.kind = "scalar"
		schema.typeName = "any"
		// Сохраняем информацию об импорте для создания type alias
		if typ.ImportPkgPath != "" {
			if typ.PkgName != "" {
				schema.importPkg = typ.PkgName
			} else if typ.ImportAlias != "" {
				schema.importPkg = typ.ImportAlias
			}
			if typ.TypeName != "" {
				schema.importName = typ.TypeName
			}
		}
		// Сохраняем тип в typeDefTs как алиас на any
		if schema.importPkg != "" && schema.importName != "" {
			typeKey := fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
			r.typeDefTs[typeKey] = schema
		}
		return
	}

	if isExcluded {
		// Проверяем time.Time по ImportPkgPath и TypeName
		if typ.ImportPkgPath == "time" && typ.TypeName == "Time" {
			schema.kind = "scalar"
			schema.typeName = "Date"
			return
		}
		// Проверяем по typeID (fallback)
		if strings.Contains(typeID, "time") && strings.Contains(typeID, "Time") {
			schema.kind = "scalar"
			schema.typeName = "Date"
			return
		}
		if strings.Contains(typeID, "UUID") || strings.HasSuffix(typ.TypeName, "UUID") {
			schema.kind = "scalar"
			schema.typeName = "string"
			return
		}
		// Для других исключаемых типов используем typeIDToTSType
		typeStr := r.typeIDToTSType(typeID)
		schema.kind = "scalar"
		schema.typeName = typeStr
		return
	}

	// Обрабатываем указатели
	if variable.NumberOfPointers > 0 {
		schema.nullable = true
	}

	// Обрабатываем базовый тип
	switch typ.Kind {
	case core.TypeKindString, core.TypeKindInt, core.TypeKindInt8, core.TypeKindInt16, core.TypeKindInt32, core.TypeKindInt64,
		core.TypeKindUint, core.TypeKindUint8, core.TypeKindUint16, core.TypeKindUint32, core.TypeKindUint64,
		core.TypeKindFloat32, core.TypeKindFloat64, core.TypeKindBool, core.TypeKindByte, core.TypeKindRune:
		// Проверяем, не является ли это time.Time с Kind=string (алиас на string)
		if typ.ImportPkgPath == "time" && typ.TypeName == "Time" {
			schema.kind = "scalar"
			schema.typeName = "Date"
			return
		}
		// Для именованных типов (type UserID int64) используем базовый TypeScript тип
		// Определяем базовый TypeScript тип на основе Kind типа
		var baseTSType string
		switch typ.Kind {
		case core.TypeKindString:
			baseTSType = "string"
		case core.TypeKindInt, core.TypeKindInt8, core.TypeKindInt16, core.TypeKindInt32, core.TypeKindInt64,
			core.TypeKindUint, core.TypeKindUint8, core.TypeKindUint16, core.TypeKindUint32, core.TypeKindUint64,
			core.TypeKindByte, core.TypeKindRune:
			baseTSType = "number"
		case core.TypeKindFloat32, core.TypeKindFloat64:
			baseTSType = "number"
		case core.TypeKindBool:
			baseTSType = "boolean"
		default:
			// Fallback: используем castTypeTs
			baseTSType = castTypeTs(typ.TypeName)
		}
		schema.kind = "scalar"
		schema.typeName = baseTSType
		// Если тип импортирован (именованный тип или алиас), сохраняем информацию об импорте
		// ВАЖНО: сохраняем типы алиасов даже если они используются только в полях структур
		// Используем PkgName для namespace (реальное имя пакета Go), fallback на ImportAlias
		if typ.TypeName != "" && typ.ImportPkgPath != "" {
			// Используем PkgName для namespace, если доступно, иначе ImportAlias
			if typ.PkgName != "" {
				schema.importPkg = typ.PkgName
			} else if typ.ImportAlias != "" {
				schema.importPkg = typ.ImportAlias
			}
			schema.importName = typ.TypeName
			// Сохраняем в typeDefTs для последующей генерации
			var typeKey string
			if schema.importPkg != "" && schema.importName != "" {
				typeKey = fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
			} else {
				typeKey = typeID
			}
			// Всегда сохраняем типы алиасов из импортированных пакетов
			r.typeDefTs[typeKey] = schema
		}
		return

	case core.TypeKindStruct:
		// Проверка на исключаемые типы уже выполнена выше, здесь обрабатываем только обычные структуры
		// Используем TypeName, если есть, иначе typeName из параметра
		if typ.TypeName != "" {
			schema.name = typ.TypeName
		} else {
			schema.name = typeName
		}
		schema.kind = "struct"
		schema.typeName = "struct"

		// Защита от циклических зависимостей: проверяем, не обрабатывается ли уже этот тип
		if processing[typeID] {
			// Циклическая зависимость обнаружена - возвращаем ссылку на тип без полей
			schema.properties = make(map[string]typeDefTs)
			return
		}

		// Помечаем тип как обрабатываемый
		processing[typeID] = true
		defer delete(processing, typeID)

		// Обрабатываем поля структуры только если они есть
		if len(typ.StructFields) > 0 {
			for _, field := range typ.StructFields {
				fieldName, inline := r.jsonName(field)
				if fieldName != "-" {
					fieldVar := &core.Variable{
						Name:             field.Name,
						TypeID:           field.TypeID,
						NumberOfPointers: field.NumberOfPointers,
						IsSlice:          field.IsSlice,
						ArrayLen:         field.ArrayLen,
						MapKeyID:         field.MapKeyID,
						MapValueID:       field.MapValueID,
					}
					fieldTags := parseTagsFromDocs(field.Docs)
					embed := r.walkVariableWithVisited(field.Name, typ.ImportPkgPath, fieldVar, fieldTags, processing, isArgument)
					if !inline {
						schema.properties[fieldName] = embed
						continue
					}
					// Inline поля - добавляем их свойства напрямую
					for eField, def := range embed.properties {
						schema.properties[eField] = def
					}
				}
			}
		}
		// Если тип импортирован, сохраняем информацию об импорте
		// Используем PkgName для namespace (реальное имя пакета Go), fallback на ImportAlias
		if typ.ImportPkgPath != "" {
			// Используем PkgName для namespace, если доступно, иначе ImportAlias
			if typ.PkgName != "" {
				schema.importPkg = typ.PkgName
			} else if typ.ImportAlias != "" {
				schema.importPkg = typ.ImportAlias
			}
			// Используем TypeName, если есть, иначе schema.name
			if typ.TypeName != "" {
				schema.importName = typ.TypeName
			} else {
				schema.importName = schema.name
			}
		}
		// Сохраняем структуру в typeDefTs для последующей генерации
		// Используем уникальный ключ: для импортированных типов - "pkg:typeName", для локальных - typeID
		var typeKey string
		if schema.importPkg != "" && schema.importName != "" {
			typeKey = fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
		} else {
			typeKey = typeID
		}
		// Сохраняем структуру в typeDefTs
		// Если тип уже существует, но новый имеет больше полей, заменяем его
		existing, exists := r.typeDefTs[typeKey]
		switch {
		case !exists:
			r.typeDefTs[typeKey] = schema
		case len(schema.properties) > len(existing.properties):
			// Если новый тип имеет больше полей, заменяем старый
			r.typeDefTs[typeKey] = schema
		case len(schema.properties) == 0 && len(existing.properties) > 0:
			// Если новый тип без полей, а старый с полями - не заменяем
			// (это может быть случай, когда структура была обработана до обработки полей)
		default:
			// В остальных случаях заменяем (новый тип может быть более полным)
			r.typeDefTs[typeKey] = schema
		}
		// Возвращаем схему для использования в typeLink()
		return

	case core.TypeKindAlias:
		// Для алиасов создаем type alias, который ссылается на базовый тип
		// ВАЖНО: не разворачиваем базовый тип, а сохраняем алиас как ссылку
		if typ.AliasOf != "" {
			// Получаем базовый тип для формирования ссылки
			baseType, baseTypeExists := r.project.Types[typ.AliasOf]
			if !baseTypeExists {
				// Базовый тип не найден - обрабатываем как обычно
				aliasVar := &core.Variable{TypeID: typ.AliasOf}
				return r.walkVariableWithVisited(typeName, pkgPath, aliasVar, varTags, processing, isArgument)
			}

			// Получаем схему базового типа для формирования ссылки
			baseVar := &core.Variable{TypeID: typ.AliasOf}
			baseSchema := r.walkVariableWithVisited("base", pkgPath, baseVar, nil, processing, isArgument)

			// Формируем ссылку на базовый тип
			var baseTypeRef string
			switch {
			case baseSchema.importPkg != "" && baseSchema.importName != "":
				// Базовый тип из другого namespace
				baseTypeRef = fmt.Sprintf("%s.%s", baseSchema.importPkg, baseSchema.importName)
			case baseType.ImportPkgPath != "":
				// Базовый тип из другого пакета, но еще не обработан
				switch {
				case baseType.PkgName != "":
					baseTypeRef = fmt.Sprintf("%s.%s", baseType.PkgName, baseType.TypeName)
				case baseType.ImportAlias != "":
					baseTypeRef = fmt.Sprintf("%s.%s", baseType.ImportAlias, baseType.TypeName)
				default:
					baseTypeRef = baseType.TypeName
				}
			default:
				// Базовый тип из того же пакета или встроенный
				baseTypeRef = baseSchema.typeLink()
			}

			// Создаем схему алиаса
			schema.kind = "scalar"
			schema.typeName = baseTypeRef
			schema.name = typ.TypeName

			// Сохраняем информацию об импорте алиаса
			if typ.ImportPkgPath != "" {
				if typ.PkgName != "" {
					schema.importPkg = typ.PkgName
				} else if typ.ImportAlias != "" {
					schema.importPkg = typ.ImportAlias
				}
				schema.importName = typ.TypeName
			}

			// Сохраняем алиас в typeDefTs
			if schema.importPkg != "" && schema.importName != "" {
				typeKey := fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
				r.typeDefTs[typeKey] = schema
			}

			return schema
		}

	case core.TypeKindArray:
		schema.kind = "array"
		schema.typeName = "array"
		schema.nullable = true
		if typ.ArrayOfID != "" {
			itemVar := &core.Variable{TypeID: typ.ArrayOfID}
			schema.properties["item"] = r.walkVariableWithVisited("item", pkgPath, itemVar, nil, processing, isArgument)
		}

	case core.TypeKindMap:
		// ВАЖНО: если это именованный map тип из текущего проекта, используем имя типа
		if typ.TypeName != "" && typ.ImportPkgPath != "" {
			// Проверяем, является ли тип из текущего проекта
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта - генерируем map с правильными типами ключа и значения
				// и сохраняем как именованный тип
				schema.kind = "map"
				schema.typeName = "map"
				schema.nullable = true
				// Генерируем типы ключа и значения
				if typ.MapKeyID != "" && typ.MapValueID != "" {
					keyVar := &core.Variable{TypeID: typ.MapKeyID}
					valueVar := &core.Variable{TypeID: typ.MapValueID}
					schema.properties["key"] = r.walkVariableWithVisited("key", pkgPath, keyVar, nil, processing, isArgument)
					schema.properties["value"] = r.walkVariableWithVisited("value", pkgPath, valueVar, nil, processing, isArgument)
				} else {
					// Если MapKeyID или MapValueID пустые, используем string для ключа и any для значения
					keyVar := &core.Variable{TypeID: "string"}
					valueVar := &core.Variable{TypeID: "any"}
					schema.properties["key"] = r.walkVariableWithVisited("key", pkgPath, keyVar, nil, processing, isArgument)
					schema.properties["value"] = r.walkVariableWithVisited("value", pkgPath, valueVar, nil, processing, isArgument)
				}
				// Сохраняем имя типа для использования в typeLink
				schema.name = typ.TypeName
				// Сохраняем информацию об импорте
				if typ.PkgName != "" {
					schema.importPkg = typ.PkgName
				} else if typ.ImportAlias != "" {
					schema.importPkg = typ.ImportAlias
				}
				schema.importName = typ.TypeName
				// Сохраняем тип в typeDefTs
				if schema.importPkg != "" && schema.importName != "" {
					typeKey := fmt.Sprintf("%s:%s", schema.importPkg, schema.importName)
					r.typeDefTs[typeKey] = schema
				}
				return schema
			}
		}
		// Для неименованных map типов генерируем map напрямую
		schema.kind = "map"
		schema.typeName = "map"
		schema.nullable = true
		if typ.MapKeyID != "" && typ.MapValueID != "" {
			keyVar := &core.Variable{TypeID: typ.MapKeyID}
			valueVar := &core.Variable{TypeID: typ.MapValueID}
			schema.properties["key"] = r.walkVariableWithVisited("key", pkgPath, keyVar, nil, processing, isArgument)
			schema.properties["value"] = r.walkVariableWithVisited("value", pkgPath, valueVar, nil, processing, isArgument)
		} else {
			// Если MapKeyID или MapValueID пустые, используем string для ключа и any для значения
			keyVar := &core.Variable{TypeID: "string"}
			valueVar := &core.Variable{TypeID: "any"}
			schema.properties["key"] = r.walkVariableWithVisited("key", pkgPath, keyVar, nil, processing, isArgument)
			schema.properties["value"] = r.walkVariableWithVisited("value", pkgPath, valueVar, nil, processing, isArgument)
		}

	case core.TypeKindInterface, core.TypeKindAny:
		schema.kind = "scalar"
		schema.name = "interface"
		schema.typeName = "any"
		schema.nullable = true

	default:
		// Fallback - используем имя типа
		schema.kind = "scalar"
		schema.typeName = typ.TypeName
		if schema.typeName == "" {
			schema.typeName = "any"
		}
	}

	return
}

// jsonName извлекает имя JSON поля из структуры
func (r *ClientRenderer) jsonName(field *core.StructField) (value string, inline bool) {
	if field.Name == "" {
		return field.Name, false
	}
	value = field.Name
	if tagValues, ok := field.Tags["json"]; ok && len(tagValues) > 0 {
		value = tagValues[0]
		if len(tagValues) == 2 {
			inline = tagValues[1] == "inline"
		}
	}
	// Если значение из тега равно "-", пропускаем поле
	if value == "-" {
		return value, false
	}
	// Проверяем, начинается ли имя с маленькой буквы (неэкспортируемое поле)
	// НО только если значение не было взято из тега json
	if len(value) > 0 && value[0] >= 'a' && value[0] <= 'z' && !inline {
		// Если значение было взято из тега, не пропускаем (тег явно указан)
		if _, ok := field.Tags["json"]; !ok {
			value = "-"
		}
	}
	return
}

// isTypeFromCurrentProject уже определен в base.go

// typeIDToTSType конвертирует typeID в TypeScript тип (упрощенная версия для базовых типов)
func (r *ClientRenderer) typeIDToTSType(typeID string) string {
	// Базовые типы
	switch typeID {
	case "string", "builtin:string":
		return "string"
	case "int", "int8", "int16", "int32", "int64", "builtin:int", "builtin:int8", "builtin:int16", "builtin:int32", "builtin:int64":
		return "number"
	case "uint", "uint8", "uint16", "uint32", "uint64", "builtin:uint", "builtin:uint8", "builtin:uint16", "builtin:uint32", "builtin:uint64":
		return "number"
	case "byte", "builtin:byte":
		return "number"
	case "rune", "builtin:rune":
		return "number"
	case "float32", "float64", "builtin:float32", "builtin:float64":
		return "number"
	case "bool", "builtin:bool":
		return "boolean"
	case "any", "interface{}", "builtin:any":
		return "any"
	case "error", "builtin:error":
		return "Error"
	case "context:Context", "context.Context":
		return "any" // context не используется в TypeScript
	}

	// Если тип найден в project.Types, используем его имя
	if typ, ok := r.project.Types[typeID]; ok {
		if typ.TypeName != "" {
			return castTypeTs(typ.TypeName)
		}
	}

	// Если typeID содержит ":", извлекаем имя типа
	if strings.Contains(typeID, ":") {
		parts := strings.SplitN(typeID, ":", 2)
		if len(parts) == 2 {
			return castTypeTs(parts[1])
		}
	}

	// По умолчанию используем any
	return "any"
}

// castTypeTs конвертирует имя Go типа в TypeScript тип
func castTypeTs(originName string) (typeName string) {
	typeName = originName
	switch originName {
	case "JSON":
		typeName = "any"
	case "bool":
		typeName = "boolean"
	case "interface":
		typeName = "any"
	case "gorm.DeletedAt":
		typeName = "Date"
	case "time.Time":
		typeName = "Date"
	case "[]byte":
		typeName = "string"
	case "float32", "float64":
		typeName = "number"
	case "byte", "int", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "time.Duration":
		typeName = "number"
	}
	if strings.HasSuffix(originName, "NullTime") {
		typeName = "Date"
	}
	if strings.HasSuffix(originName, "RawMessage") {
		typeName = "any"
	}
	if strings.HasSuffix(originName, "UUID") {
		typeName = "string"
	}
	if strings.HasSuffix(originName, "Decimal") {
		typeName = "number"
	}
	return
}

// annotationIsSet проверяет, установлена ли аннотация
func annotationIsSet(annotations map[string]string, key string) bool {
	if annotations == nil {
		return false
	}
	_, ok := annotations[key]
	return ok
}

// annotationValue возвращает значение аннотации
func annotationValue(annotations map[string]string, key, defaultValue string) string {
	if annotations == nil {
		return defaultValue
	}
	if value, ok := annotations[key]; ok {
		return value
	}
	return defaultValue
}

// parseTagsFromDocs парсит теги из документации
func parseTagsFromDocs(docs []string) map[string]string {
	result := make(map[string]string)
	for _, doc := range docs {
		// Простая реализация - ищем теги в формате @tag value
		if strings.HasPrefix(doc, "@") {
			parts := strings.Fields(doc)
			if len(parts) >= 2 {
				tagName := strings.TrimPrefix(parts[0], "@")
				result[tagName] = parts[1]
			}
		}
	}
	return result
}

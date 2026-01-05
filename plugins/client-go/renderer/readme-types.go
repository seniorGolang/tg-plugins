// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path/filepath"
	"strings"

	"tgp/core"
)

// typeUsage содержит информацию об использовании типа
type typeUsage struct {
	typeName     string
	pkgPath      string
	fullTypeName string
	locations    []string
}

// collectStructTypes собирает все используемые типы структур
func (r *ClientRenderer) collectStructTypes() map[string]*typeUsage {
	typeUsages := make(map[string]*typeUsage)

	for _, contract := range r.project.Contracts {
		for _, method := range contract.Methods {
			// Параметры запроса
			args := r.argsWithoutContext(method)
			for _, arg := range args {
				if structType, typeName, pkg := r.getStructType(arg.TypeID, contract.PkgPath); structType != nil {
					// Формируем ключ
					keyTypeName := typeName
					if typeName == "" {
						typeName = arg.Name
						keyTypeName = arg.Name
					}
					// Если typeName содержит точку (импортированный тип), извлекаем только имя типа для ключа
					if strings.Contains(keyTypeName, ".") {
						parts := strings.Split(keyTypeName, ".")
						keyTypeName = parts[len(parts)-1]
					}
					key := fmt.Sprintf("%s.%s", pkg, keyTypeName)

					// Формируем полное имя типа для ключа
					fullTypeNameForKey := typeName
					if fullTypeNameForKey == "" {
						fullTypeNameForKey = arg.Name
					}

					if _, ok := typeUsages[key]; !ok {
						typeUsages[key] = &typeUsage{
							typeName:     keyTypeName,
							pkgPath:      pkg,
							fullTypeName: fullTypeNameForKey,
							locations:    make([]string, 0),
						}
					}
					location := fmt.Sprintf("%s.%s.%s", contract.Name, method.Name, arg.Name)
					typeUsages[key].locations = append(typeUsages[key].locations, location)
				}
			}

			// Результаты
			results := r.resultsWithoutError(method)
			for _, result := range results {
				if structType, typeName, pkg := r.getStructType(result.TypeID, contract.PkgPath); structType != nil {
					// Формируем ключ
					keyTypeName := typeName
					if typeName == "" {
						typeName = result.Name
						keyTypeName = result.Name
					}
					// Если typeName содержит точку (импортированный тип), извлекаем только имя типа для ключа
					if strings.Contains(keyTypeName, ".") {
						parts := strings.Split(keyTypeName, ".")
						keyTypeName = parts[len(parts)-1]
					}
					key := fmt.Sprintf("%s.%s", pkg, keyTypeName)

					// Формируем полное имя типа для ключа
					fullTypeNameForKey := typeName
					if fullTypeNameForKey == "" {
						fullTypeNameForKey = result.Name
					}

					if _, ok := typeUsages[key]; !ok {
						typeUsages[key] = &typeUsage{
							typeName:     keyTypeName,
							pkgPath:      pkg,
							fullTypeName: fullTypeNameForKey,
							locations:    make([]string, 0),
						}
					}
					location := fmt.Sprintf("%s.%s.%s", contract.Name, method.Name, result.Name)
					typeUsages[key].locations = append(typeUsages[key].locations, location)
				}
			}
		}
	}

	return typeUsages
}

// getStructType получает структуру из типа (включая импортированные)
func (r *ClientRenderer) getStructType(typeID, pkgPath string) (structType *core.Type, typeName string, pkg string) {
	typ, ok := r.project.Types[typeID]
	if !ok {
		return nil, "", ""
	}

	// Проверяем, является ли тип структурой
	if typ.Kind != core.TypeKindStruct || typ.TypeName == "" {
		return nil, "", ""
	}

	// Это структура
	typeName = typ.TypeName
	pkg = typ.ImportPkgPath
	if pkg == "" {
		pkg = pkgPath
	}

	return typ, typeName, pkg
}

// goTypeStringFromVariable возвращает строковое представление Go типа из Variable
func (r *ClientRenderer) goTypeStringFromVariable(variable *core.Variable, pkgPath string) string {
	// Обрабатываем массивы и слайсы
	if variable.IsSlice || variable.ArrayLen > 0 {
		elemType := r.goTypeString(variable.TypeID, pkgPath)
		if variable.IsSlice {
			return fmt.Sprintf("[]%s", elemType)
		}
		return fmt.Sprintf("[%d]%s", variable.ArrayLen, elemType)
	}

	// Обрабатываем map
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		keyType := r.goTypeString(variable.MapKeyID, pkgPath)
		valueType := r.goTypeString(variable.MapValueID, pkgPath)
		return fmt.Sprintf("map[%s]%s", keyType, valueType)
	}

	// Базовый тип
	return r.goTypeString(variable.TypeID, pkgPath)
}

// goTypeStringFromStructField возвращает строковое представление Go типа из StructField
func (r *ClientRenderer) goTypeStringFromStructField(field *core.StructField, pkgPath string) string {
	// Обрабатываем массивы и слайсы
	if field.IsSlice || field.ArrayLen > 0 {
		elemType := r.goTypeString(field.TypeID, pkgPath)
		if field.IsSlice {
			return fmt.Sprintf("[]%s", elemType)
		}
		return fmt.Sprintf("[%d]%s", field.ArrayLen, elemType)
	}

	// Обрабатываем map
	if field.MapKeyID != "" && field.MapValueID != "" {
		keyType := r.goTypeString(field.MapKeyID, pkgPath)
		valueType := r.goTypeString(field.MapValueID, pkgPath)
		return fmt.Sprintf("map[%s]%s", keyType, valueType)
	}

	// Базовый тип
	return r.goTypeString(field.TypeID, pkgPath)
}

// goTypeString возвращает строковое представление Go типа
func (r *ClientRenderer) goTypeString(typeID, pkgPath string) string {
	typ, ok := r.project.Types[typeID]
	if !ok {
		// Тип не найден - возможно, это встроенный тип
		if r.isBuiltinType(typeID) {
			return typeID
		}

		// Если typeID содержит ":", это импортированный тип
		if strings.Contains(typeID, ":") {
			parts := strings.SplitN(typeID, ":", 2)
			if len(parts) == 2 {
				importPkg := parts[0]
				typeName := parts[1]
				baseName := filepath.Base(importPkg)
				if baseName == "" {
					baseName = importPkg
				}
				return fmt.Sprintf("%s.%s", baseName, typeName)
			}
		}

		return typeID
	}

	// Тип найден в project.Types

	// Сначала проверяем импортированные типы (имеют ImportPkgPath)
	if typ.ImportPkgPath != "" {
		alias := typ.ImportAlias
		if alias == "" {
			alias = filepath.Base(typ.ImportPkgPath)
		}

		typeName := typ.TypeName
		if typeName == "" {
			if strings.Contains(typeID, ":") {
				parts := strings.SplitN(typeID, ":", 2)
				if len(parts) == 2 {
					typeName = parts[1]
				}
			}
		}

		if typeName != "" {
			return fmt.Sprintf("%s.%s", alias, typeName)
		}
	}

	// Обрабатываем в зависимости от Kind
	switch typ.Kind {
	case core.TypeKindString, core.TypeKindInt, core.TypeKindInt8, core.TypeKindInt16, core.TypeKindInt32, core.TypeKindInt64,
		core.TypeKindUint, core.TypeKindUint8, core.TypeKindUint16, core.TypeKindUint32, core.TypeKindUint64,
		core.TypeKindFloat32, core.TypeKindFloat64, core.TypeKindBool, core.TypeKindByte, core.TypeKindRune, core.TypeKindError, core.TypeKindAny:
		if typ.ImportPkgPath != "" && typ.TypeName != "" {
			alias := typ.ImportAlias
			if alias == "" {
				alias = filepath.Base(typ.ImportPkgPath)
			}
			return fmt.Sprintf("%s.%s", alias, typ.TypeName)
		}
		return string(typ.Kind)
	case core.TypeKindStruct:
		structName := typ.TypeName
		if structName == "" {
			if strings.Contains(typeID, ":") {
				parts := strings.SplitN(typeID, ":", 2)
				if len(parts) == 2 {
					structName = parts[1]
				}
			}
		}

		if typ.ImportPkgPath != "" {
			alias := typ.ImportAlias
			if alias == "" {
				alias = filepath.Base(typ.ImportPkgPath)
			}
			if structName != "" {
				return fmt.Sprintf("%s.%s", alias, structName)
			}
		}

		if structName != "" {
			return structName
		}
	case core.TypeKindArray:
		if typ.IsSlice {
			elemType := r.goTypeString(typ.ArrayOfID, pkgPath)
			return fmt.Sprintf("[]%s", elemType)
		}
		elemType := r.goTypeString(typ.ArrayOfID, pkgPath)
		return fmt.Sprintf("[%d]%s", typ.ArrayLen, elemType)
	case core.TypeKindMap:
		keyType := r.goTypeString(typ.MapKeyID, pkgPath)
		valueType := r.goTypeString(typ.MapValueID, pkgPath)
		return fmt.Sprintf("map[%s]%s", keyType, valueType)
	}

	// Если TypeName задан, используем его
	if typ.TypeName != "" {
		if typ.ImportPkgPath != "" {
			alias := typ.ImportAlias
			if alias == "" {
				alias = filepath.Base(typ.ImportPkgPath)
			}
			return fmt.Sprintf("%s.%s", alias, typ.TypeName)
		}
		return typ.TypeName
	}

	// Если ничего не помогло, пытаемся извлечь из typeID
	if strings.Contains(typeID, ":") {
		parts := strings.SplitN(typeID, ":", 2)
		if len(parts) == 2 {
			baseName := filepath.Base(parts[0])
			if baseName == "" {
				baseName = parts[0]
			}
			return fmt.Sprintf("%s.%s", baseName, parts[1])
		}
	}

	return typeID
}

// jsonName извлекает имя JSON поля из тегов
func (r *ClientRenderer) jsonName(field *core.StructField) (value string, inline bool) {
	if tagValues, ok := field.Tags["json"]; ok {
		for _, val := range tagValues {
			if val == "inline" {
				inline = true
				continue
			}
			if val != "omitempty" && val != "-" {
				value = val
			}
		}
	}
	if value == "" {
		value = field.Name
	}
	return
}

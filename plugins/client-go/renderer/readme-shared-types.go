// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"sort"
	"strings"

	"tgp/internal/markdown"

	"tgp/shared"
)

// renderAllTypes генерирует секцию "Общие типы" для всех типов, используемых в контрактах
func (r *ClientRenderer) renderAllTypes(md *markdown.Markdown, allTypes map[string]*typeUsage) {
	sharedTypesAnchor := generateAnchor("Общие типы")
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", sharedTypesAnchor))
	md.LF()
	md.H2("Общие типы")
	md.PlainText("Типы данных, используемые в клиенте. Типы, используемые в нескольких методах, описаны здесь для избежания дублирования.")
	md.LF()

	// Сортируем типы по имени для консистентности
	typeKeys := make([]string, 0, len(allTypes))
	for key := range allTypes {
		typeKeys = append(typeKeys, key)
	}
	sort.Strings(typeKeys)

	for _, key := range typeKeys {
		usage := allTypes[key]
		// Получаем структуру типа
		typ, ok := r.project.Types[key]
		if !ok {
			// Пытаемся найти тип по pkgPath и typeName
			for _, t := range r.project.Types {
				if t.ImportPkgPath == usage.pkgPath && t.TypeName == usage.typeName {
					typ = t
					break
				}
			}
		}

		// Проверяем, является ли тип из внешней библиотеки
		isExternal := typ != nil && typ.ImportPkgPath != "" && !r.isTypeFromCurrentProject(typ.ImportPkgPath)
		
		if typ != nil && typ.Kind == shared.TypeKindStruct && !isExternal {
			// Локальная структура - выводим таблицу полей
			r.renderStructTypeTable(md, typ, usage.fullTypeName, usage.pkgPath)
		} else {
			// Если тип не найден, не структура или из внешней библиотеки, выводим только заголовок
			typeAnchor := generateAnchor(usage.fullTypeName)
			md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", typeAnchor))
			md.LF()
			md.H4(usage.fullTypeName)
			
			// Если тип из внешней библиотеки, добавляем информацию о библиотеке
			if isExternal {
				importPath := typ.ImportPkgPath
				libURL := fmt.Sprintf("https://pkg.go.dev/%s", importPath)
				md.PlainText(fmt.Sprintf("Тип из внешней библиотеки: [%s](%s)", importPath, libURL))
				md.LF()
			}
			
			md.LF()
		}
	}

	md.HorizontalRule()
}

// getTypeLinkFromStructField возвращает ссылку на тип для поля структуры
func (r *ClientRenderer) getTypeLinkFromStructField(field *shared.StructField, pkgPath string) string {
	switch {
	case field.IsSlice || field.ArrayLen > 0:
		// Для массивов/слайсов - ссылка на тип элемента (без [])
		return r.getTypeLink(field.TypeID, pkgPath)
	case field.MapKeyID != "" && field.MapValueID != "":
		// Для map - две ссылки: на ключ и значение (примитивы пропускаем)
		keyLink := r.getTypeLink(field.MapKeyID, pkgPath)
		valueLink := r.getTypeLink(field.MapValueID, pkgPath)
		links := []string{}
		if keyLink != "-" {
			links = append(links, keyLink)
		}
		if valueLink != "-" {
			links = append(links, valueLink)
		}
		if len(links) > 0 {
			return strings.Join(links, ", ")
		}
		return "-"
	default:
		// Обычный тип
		return r.getTypeLink(field.TypeID, pkgPath)
	}
}

// getTypeLinkFromVariable возвращает ссылку на тип для переменной (параметр или возвращаемое значение)
func (r *ClientRenderer) getTypeLinkFromVariable(variable *shared.Variable, pkgPath string) string {
	switch {
	case variable.IsSlice || variable.ArrayLen > 0:
		// Для массивов/слайсов - ссылка на тип элемента (без [])
		return r.getTypeLink(variable.TypeID, pkgPath)
	case variable.MapKeyID != "" && variable.MapValueID != "":
		// Для map - две ссылки: на ключ и значение (примитивы пропускаем)
		keyLink := r.getTypeLink(variable.MapKeyID, pkgPath)
		valueLink := r.getTypeLink(variable.MapValueID, pkgPath)
		links := []string{}
		if keyLink != "-" {
			links = append(links, keyLink)
		}
		if valueLink != "-" {
			links = append(links, valueLink)
		}
		if len(links) > 0 {
			return strings.Join(links, ", ")
		}
		return "-"
	default:
		// Обычный тип
		return r.getTypeLink(variable.TypeID, pkgPath)
	}
}

// getTypeLink возвращает ссылку на тип или "-" если это примитив или локальный тип
func (r *ClientRenderer) getTypeLink(typeID, pkgPath string) string {
	// Проверяем, является ли тип примитивом
	if r.isBuiltinType(typeID) {
		return "-"
	}

	// Проверяем, является ли тип структурой
	if structType, typeName, _ := r.getStructType(typeID, pkgPath); structType != nil {
		// Это структура - создаём ссылку на таблицу типа
		if typeName == "" {
			// Если typeName пустой, используем typeID
			typeName = typeID
		}
		typeAnchor := generateAnchor(typeName)
		// Получаем строковое представление типа для отображения
		typeStr := r.goTypeString(typeID, pkgPath)
		return fmt.Sprintf("[%s](#%s)", typeStr, typeAnchor)
	}

	// Проверяем, является ли тип внешним
	typ, ok := r.project.Types[typeID]
	if ok && typ.ImportPkgPath != "" && !r.isTypeFromCurrentProject(typ.ImportPkgPath) {
		// Внешний тип - создаём ссылку на библиотеку
		importPath := typ.ImportPkgPath
		libURL := fmt.Sprintf("https://pkg.go.dev/%s", importPath)
		typeStr := r.goTypeString(typeID, pkgPath)
		return fmt.Sprintf("[%s](%s)", typeStr, libURL)
	}

	// Локальный тип или примитив - без ссылки
	return "-"
}

// renderStructTypeTable генерирует таблицу для типа структуры
// Вызывается только для локальных типов (не из внешних библиотек)
func (r *ClientRenderer) renderStructTypeTable(md *markdown.Markdown, structType *shared.Type, typeName string, pkgPath string) {
	// Заголовок таблицы типа
	typeAnchor := generateAnchor(typeName)
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", typeAnchor))
	md.LF()
	md.H4(typeName)

	// Собираем данные для таблицы полей
	rows := make([][]string, 0)
	for _, field := range structType.StructFields {
		fieldName, _ := r.jsonName(field)
		if fieldName == "-" {
			continue
		}

		// Тип поля
		typeStr := r.goTypeStringFromStructField(field, pkgPath)

		// Описание
		fieldTags := r.parseTagsFromDocs(strings.Join(field.Docs, "\n"))
		fieldDesc := fieldTags[tagDesc]

		// Required
		isRequired := fieldTags[tagRequired] != ""
		requiredStr := "Нет"
		if isRequired {
			requiredStr = "Да"
		}

		// Nullable
		isNullable := field.NumberOfPointers > 0
		nullableStr := "Нет"
		if isNullable {
			nullableStr = "Да"
		}

		// Omitempty
		hasOmitempty := false
		if tagValues, ok := field.Tags["json"]; ok {
			for _, val := range tagValues {
				if val == "omitempty" {
					hasOmitempty = true
					break
				}
			}
		}
		omitemptyStr := "Нет"
		if hasOmitempty {
			omitemptyStr = "Да"
		}

		// Ссылка на тип
		typeLink := r.getTypeLinkFromStructField(field, pkgPath)

		// Формируем строку таблицы
		if fieldDesc != "" {
			rows = append(rows, []string{
				markdown.Code(fieldName),
				markdown.Code(typeStr),
				fieldDesc,
				requiredStr,
				nullableStr,
				omitemptyStr,
				typeLink,
			})
		} else {
			rows = append(rows, []string{
				markdown.Code(fieldName),
				markdown.Code(typeStr),
				requiredStr,
				nullableStr,
				omitemptyStr,
				typeLink,
			})
		}
	}

	// Формируем заголовки
	var headers []string
	hasDescriptions := false
	for _, field := range structType.StructFields {
		fieldTags := r.parseTagsFromDocs(strings.Join(field.Docs, "\n"))
		fieldDesc := fieldTags[tagDesc]
		if fieldDesc != "" {
			hasDescriptions = true
			break
		}
	}

	if hasDescriptions {
		headers = []string{"Поле", "Тип", "Описание", "Обязательное", "Nullable", "Omitempty", "Ссылка на тип"}
	} else {
		headers = []string{"Поле", "Тип", "Обязательное", "Nullable", "Omitempty", "Ссылка на тип"}
	}

	tableSet := markdown.TableSet{
		Header: headers,
		Rows:   rows,
	}
	md.Table(tableSet)
	md.LF()
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"slices"
	"strings"

	"tgp/internal/parser"
	"tgp/internal/tags"
)

// expandInlineFields разворачивает inline поля в списке переменных.
func (r *contractRenderer) expandInlineFields(vars []*parser.Variable, methodTags tags.DocTags) []*parser.Variable {

	result := make([]*parser.Variable, 0, len(vars))
	for _, v := range vars {
		// Проверяем, является ли поле inline через аннотации метода
		if r.isInlineField(v, methodTags) {
			// Разворачиваем inline структуру
			expanded := r.expandInlineStruct(v)
			result = append(result, expanded...)
		} else {
			result = append(result, v)
		}
	}
	return result
}

// isInlineField проверяет, является ли поле inline.
func (r *contractRenderer) isInlineField(v *parser.Variable, methodTags tags.DocTags) bool {

	// Проверяем теги в аннотациях метода
	for key, value := range methodTags.Sub(v.Name) {
		if key == "tag" {
			// Формат: tag:json:fieldName,inline|tag:xml:fieldName
			if list := strings.Split(value, "|"); len(list) > 0 {
				for _, item := range list {
					if tokens := strings.Split(item, ":"); len(tokens) >= 2 {
						tagName := tokens[0]
						tagValue := strings.Join(tokens[1:], ":")
						// Проверяем json тег на inline
						if tagName == "json" && (tagValue == "inline" || strings.Contains(tagValue, ",inline")) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// expandInlineStruct разворачивает inline структуру в список полей.
func (r *contractRenderer) expandInlineStruct(v *parser.Variable) []*parser.Variable {

	// Получаем тип из project.Types
	typ, ok := r.project.Types[v.TypeID]
	if !ok {
		// Тип не найден, возвращаем как есть
		return []*parser.Variable{v}
	}

	// Проверяем, что это структура
	if typ.Kind != parser.TypeKindStruct {
		return []*parser.Variable{v}
	}

	// Если это алиас структуры, получаем базовый тип
	if typ.Kind == parser.TypeKindAlias && typ.AliasOf != "" {
		baseTyp, ok := r.project.Types[typ.AliasOf]
		if ok && baseTyp.Kind == parser.TypeKindStruct {
			typ = baseTyp
		}
	}

	// Разворачиваем поля структуры
	result := make([]*parser.Variable, 0)
	for _, field := range typ.StructFields {
		// Проверяем, является ли само поле inline (вложенные inline)
		fieldVar := &parser.Variable{
			Name:             field.Name,
			TypeID:           field.TypeID,
			NumberOfPointers: field.NumberOfPointers,
			IsSlice:          field.IsSlice,
			ArrayLen:         field.ArrayLen,
			IsEllipsis:       field.IsEllipsis,
			ElementPointers:  field.ElementPointers,
			MapKeyID:         field.MapKeyID,
			MapValueID:       field.MapValueID,
		}

		// Проверяем теги поля на inline
		isInline := false
		if field.Tags != nil {
			if jsonTags, ok := field.Tags["json"]; ok {
				isInline = slices.ContainsFunc(jsonTags, func(tag string) bool {
					return tag == "inline" || strings.Contains(tag, ",inline")
				})
			}
		}

		if isInline {
			// Рекурсивно разворачиваем вложенные inline поля
			expanded := r.expandInlineStruct(fieldVar)
			result = append(result, expanded...)
		} else {
			result = append(result, fieldVar)
		}
	}

	return result
}

// ArgsFieldsWithoutContext возвращает аргументы без context, обрабатывая inline поля.
func (r *contractRenderer) ArgsFieldsWithoutContext(method *parser.Method) []*parser.Variable {

	argVars := r.expandInlineFields(method.Args, method.Annotations)
	// Убираем context, если он первый
	if len(argVars) > 0 && argVars[0].TypeID == "context:Context" {
		return argVars[1:]
	}
	return argVars
}

// ResultFieldsWithoutError возвращает результаты без error, обрабатывая inline поля.
func (r *contractRenderer) ResultFieldsWithoutError(method *parser.Method) []*parser.Variable {

	resultVars := r.expandInlineFields(method.Results, method.Annotations)
	// Убираем error, если он последний
	if len(resultVars) > 0 && resultVars[len(resultVars)-1].TypeID == "error" {
		return resultVars[:len(resultVars)-1]
	}
	return resultVars
}

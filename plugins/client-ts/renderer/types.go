// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import "fmt"

// typeDefTs представляет определение типа для TypeScript
type typeDefTs struct {
	name       string
	kind       string
	typeName   string
	nullable   bool
	value      interface{}
	properties map[string]typeDefTs
	// Для импортированных типов храним информацию о пакете и имени типа
	importPkg  string // Пакет импорта (например, "dto")
	importName string // Имя типа в пакете (например, "SomeStruct")
}

func (def typeDefTs) def() (prop string) {
	switch def.kind {
	case "map":
		if def.properties == nil {
			return "Record<string, any>"
		}
		key := def.properties["key"]
		value := def.properties["value"]
		keyType := "string"
		valueType := "any"
		if key.kind != "" {
			keyType = castTypeTs(key.typeLink())
		}
		if value.kind != "" {
			valueType = castTypeTs(value.typeLink())
		}
		return fmt.Sprintf("Record<%s, %s>", keyType, valueType)
	case "array":
		if def.properties == nil {
			return "any[]"
		}
		item := def.properties["item"]
		itemType := "any"
		if item.kind != "" {
			itemType = item.typeLink()
		}
		return fmt.Sprintf("%s[]", itemType)
	case "struct":
		return def.name
	case "scalar":
		// Для скалярных типов (алиасов) используем базовый TypeScript тип
		// Если тип импортирован, используем namespace (например, "dto.UserID")
		if def.importPkg != "" && def.importName != "" {
			return fmt.Sprintf("%s.%s", def.importPkg, def.importName)
		}
		return def.typeName
	default:
		return castTypeTs(def.kind)
	}
}

func (def typeDefTs) typeLink() (link string) {
	switch def.kind {
	case "map":
		// Если это именованный map тип из текущего проекта, используем ссылку на тип
		if def.importPkg != "" && def.importName != "" {
			return fmt.Sprintf("%s.%s", def.importPkg, def.importName)
		}
		// Для неименованных map типов генерируем Record<...>
		if def.properties == nil {
			return "Record<string, any>"
		}
		keyType := "string"
		valueType := "any"
		if key, ok := def.properties["key"]; ok {
			keyType = castTypeTs(key.typeLink())
		}
		if value, ok := def.properties["value"]; ok {
			valueType = castTypeTs(value.typeLink())
		}
		return fmt.Sprintf("Record<%s, %s>", keyType, valueType)
	case "array":
		if def.properties == nil {
			return "any[]"
		}
		itemType := "any"
		if item, ok := def.properties["item"]; ok {
			itemType = castTypeTs(item.typeLink())
		}
		return fmt.Sprintf("%s[]", itemType)
	case "scalar":
		// Для скалярных типов (алиасов) используем базовый TypeScript тип
		// Специальная обработка для time.Time -> Date
		if def.importPkg == "time" && def.importName == "Time" {
			return "Date"
		}
		// Если тип импортирован, используем namespace (например, "dto.UserID")
		if def.importPkg != "" && def.importName != "" {
			return fmt.Sprintf("%s.%s", def.importPkg, def.importName)
		}
		return def.typeName
	case "struct":
		// Если есть информация об импорте, используем namespace (например, "dto.SomeStruct")
		if def.importPkg != "" && def.importName != "" {
			return fmt.Sprintf("%s.%s", def.importPkg, def.importName)
		}
		// Если name пустой, пытаемся использовать importName
		if def.name == "" && def.importName != "" {
			return def.importName
		}
		// Если все еще пустой, возвращаем "any" как fallback
		if def.name == "" {
			return "any"
		}
		return def.name
	case "":
		// Если kind пустой, это может быть неинициализированный тип
		// Пытаемся использовать name или typeName
		if def.name != "" {
			return def.name
		}
		if def.typeName != "" {
			return def.typeName
		}
		return "any"
	default:
		return castTypeTs(def.name)
	}
}


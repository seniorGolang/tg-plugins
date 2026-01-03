// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package types

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
)

// Generator генерирует типы для Go кода.
type Generator struct {
	project   *core.Project
	srcFile   SrcFile
	typeCache map[string]*Statement
}

// NewGenerator создает новый генератор типов.
func NewGenerator(project *core.Project, srcFile SrcFile) *Generator {
	return &Generator{
		project:   project,
		srcFile:   srcFile,
		typeCache: make(map[string]*Statement),
	}
}

// FieldTypeFromVariable конвертирует тип из Variable в код jennifer.
func (g *Generator) FieldTypeFromVariable(variable *core.Variable, allowEllipsis bool) *Statement {
	c := &Statement{}

	// Обрабатываем ellipsis
	if variable.IsEllipsis && allowEllipsis {
		c.Op("...")
		if variable.TypeID != "" {
			return c.Add(g.FieldType(variable.TypeID, variable.NumberOfPointers, false))
		}
		return c
	}

	// Обрабатываем массивы и слайсы
	if variable.IsSlice || variable.ArrayLen > 0 {
		for i := 0; i < variable.NumberOfPointers; i++ {
			c.Op("*")
		}
		if variable.IsSlice {
			c.Index()
		} else {
			c.Index(Lit(variable.ArrayLen))
		}
		if variable.TypeID != "" {
			return c.Add(g.FieldType(variable.TypeID, variable.ElementPointers, false))
		}
		return c
	}

	// Обрабатываем map
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		for i := 0; i < variable.NumberOfPointers; i++ {
			c.Op("*")
		}
		keyType := g.FieldType(variable.MapKeyID, variable.MapKeyPointers, false)
		valueType := g.FieldType(variable.MapValueID, variable.ElementPointers, false)
		return c.Map(keyType).Add(valueType)
	}

	// Базовый тип
	return c.Add(g.FieldType(variable.TypeID, variable.NumberOfPointers, false))
}

// FieldType конвертирует тип из core в код jennifer.
func (g *Generator) FieldType(typeID string, numberOfPointers int, allowEllipsis bool) *Statement {

	// Проверяем кэш
	cacheKey := fmt.Sprintf("%s:%d:%v", typeID, numberOfPointers, allowEllipsis)
	if cached, ok := g.typeCache[cacheKey]; ok {
		if cacheLogger != nil {
			cacheLogger.OnCacheHit()
		}
		return cached
	}

	// Логируем промах кэша
	if cacheLogger != nil {
		cacheLogger.OnCacheMiss()
	}

	// Генерируем тип
	result := g.fieldTypeImpl(typeID, numberOfPointers, allowEllipsis)

	// Сохраняем в кэш
	g.typeCache[cacheKey] = result
	return result
}

// fieldTypeImpl реализует генерацию типа без кэширования.
func (g *Generator) fieldTypeImpl(typeID string, numberOfPointers int, allowEllipsis bool) *Statement {

	c := &Statement{}

	// Добавляем указатели
	for i := 0; i < numberOfPointers; i++ {
		c.Op("*")
	}

	// Получаем тип из проекта
	typ, ok := g.project.Types[typeID]
	if !ok {
		return g.fieldTypeFromID(typeID, c)
	}

	// Обрабатываем ellipsis
	if typ.IsEllipsis && allowEllipsis {
		c.Op("...")
		if typ.ArrayOfID != "" {
			return c.Add(g.FieldType(typ.ArrayOfID, 0, false))
		}
		return c
	}

	// Обрабатываем в зависимости от вида типа
	switch typ.Kind {
	case core.TypeKindArray:
		return g.fieldTypeArray(typ, c)
	case core.TypeKindMap:
		return g.fieldTypeMap(typ, c)
	case core.TypeKindChan:
		return g.fieldTypeChan(typ, c)
	case core.TypeKindStruct:
		return g.fieldTypeStruct(typ, c)
	case core.TypeKindInterface:
		return g.fieldTypeInterface(typ, c)
	case core.TypeKindFunction:
		return g.fieldTypeFunction(typ, c)
	case core.TypeKindAlias:
		return g.fieldTypeAlias(typ, numberOfPointers, allowEllipsis, c)
	default:
		return g.fieldTypePrimitive(typ, typeID, c)
	}
}

// fieldTypeFromID извлекает тип из typeID, если он не найден в project.Types.
func (g *Generator) fieldTypeFromID(typeID string, c *Statement) *Statement {
	if strings.Contains(typeID, ":") {
		parts := strings.SplitN(typeID, ":", 2)
		if len(parts) == 2 && parts[1] != "" {
			pkgPath := parts[0]
			typeName := parts[1]
			g.srcFile.ImportName(pkgPath, filepath.Base(pkgPath))
			return c.Qual(pkgPath, typeName)
		}
	}
	return c.Id(typeID)
}

// fieldTypeArray генерирует код для массива или слайса.
func (g *Generator) fieldTypeArray(typ *core.Type, c *Statement) *Statement {
	// Если это именованный тип, который определен как массив
	if typ.TypeName != "" && (typ.ImportPkgPath != "" || typ.PkgName != "") {
		return g.fieldTypeNamed(typ, c)
	}

	// Обычный массив/слайс
	switch {
	case typ.IsSlice:
		c.Index()
	case typ.ArrayLen > 0:
		c.Index(Lit(typ.ArrayLen))
	default:
		c.Index()
	}
	if typ.ArrayOfID != "" {
		return c.Add(g.FieldType(typ.ArrayOfID, 0, false))
	}
	return c
}

// fieldTypeMap генерирует код для map.
func (g *Generator) fieldTypeMap(typ *core.Type, c *Statement) *Statement {
	if typ.MapKeyID != "" && typ.MapValueID != "" {
		keyType := g.FieldType(typ.MapKeyID, typ.MapKeyPointers, false)
		valueType := g.FieldType(typ.MapValueID, typ.ElementPointers, false)
		return c.Map(keyType).Add(valueType)
	}
	return c
}

// fieldTypeChan генерирует код для канала.
func (g *Generator) fieldTypeChan(typ *core.Type, c *Statement) *Statement {
	switch typ.ChanDirection {
	case 1: // send only
		c.Chan().Op("<-")
	case 2: // receive only
		c.Op("<-").Chan()
	default: // both
		c.Chan()
	}
	if typ.ChanOfID != "" {
		return c.Add(g.FieldType(typ.ChanOfID, 0, false))
	}
	return c
}

// fieldTypeStruct генерирует код для структуры.
func (g *Generator) fieldTypeStruct(typ *core.Type, c *Statement) *Statement {
	return g.fieldTypeNamed(typ, c)
}

// fieldTypeInterface генерирует код для интерфейса.
func (g *Generator) fieldTypeInterface(typ *core.Type, c *Statement) *Statement {
	return g.fieldTypeNamed(typ, c)
}

// fieldTypeFunction генерирует код для функционального типа.
func (g *Generator) fieldTypeFunction(typ *core.Type, c *Statement) *Statement {
	args := make([]Code, 0, len(typ.FunctionArgs))
	for _, arg := range typ.FunctionArgs {
		argType := g.FieldTypeFromVariable(arg, false)
		args = append(args, Id(toLowerCamel(arg.Name)).Add(argType))
	}
	results := make([]Code, 0, len(typ.FunctionResults))
	for _, res := range typ.FunctionResults {
		resType := g.FieldTypeFromVariable(res, false)
		results = append(results, resType)
	}
	return c.Func().Params(args...).Params(results...)
}

// fieldTypeAlias генерирует код для алиаса типа.
func (g *Generator) fieldTypeAlias(typ *core.Type, numberOfPointers int, allowEllipsis bool, c *Statement) *Statement {
	if typ.AliasOf != "" {
		return g.FieldType(typ.AliasOf, numberOfPointers, allowEllipsis)
	}
	return c.Id(typ.TypeName)
}

// fieldTypePrimitive генерирует код для примитивных типов.
func (g *Generator) fieldTypePrimitive(typ *core.Type, typeID string, c *Statement) *Statement {
	// Если у типа есть ImportPkgPath и TypeName, это именованный тип из другого пакета
	if typ.ImportPkgPath != "" && typ.TypeName != "" {
		return g.fieldTypeNamed(typ, c)
	}

	// Встроенный базовый тип
	if typ.Kind != "" {
		return c.Id(string(typ.Kind))
	}
	return c.Id(typeID)
}

// fieldTypeNamed генерирует код для именованного типа.
func (g *Generator) fieldTypeNamed(typ *core.Type, c *Statement) *Statement {
	if typ.ImportPkgPath != "" {
		packageName := typ.PkgName
		if packageName == "" {
			packageName = filepath.Base(typ.ImportPkgPath)
		}
		if typ.ImportAlias != "" && typ.ImportAlias != packageName {
			g.srcFile.ImportAlias(typ.ImportPkgPath, typ.ImportAlias)
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
		g.srcFile.ImportName(typ.ImportPkgPath, packageName)
		return c.Qual(typ.ImportPkgPath, typ.TypeName)
	}
	return c.Id(typ.TypeName)
}

// FuncDefinitionParams генерирует параметры или результаты метода.
func (g *Generator) FuncDefinitionParams(vars []*core.Variable) *Statement {
	c := &Statement{}
	c.ListFunc(func(gr *Group) {
		for _, v := range vars {
			typeCode := g.FieldTypeFromVariable(v, true)
			varName := v.Name
			if varName == "" {
				varName = "_"
			} else {
				varName = toLowerCamel(varName)
			}
			gr.Id(varName).Add(typeCode)
		}
	})
	return c
}

// toLowerCamel конвертирует строку в lowerCamelCase.
func toLowerCamel(s string) string {

	if s == "" {
		return s
	}
	// Проверяем, все ли символы в верхнем регистре
	allUpper := true
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			allUpper = false
			break
		}
	}
	if allUpper {
		return s
	}
	// Если первый символ в верхнем регистре, делаем его нижним
	runes := []rune(s)
	if len(runes) > 0 && runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] = unicode.ToLower(runes[0])
		s = string(runes)
	}
	// Применяем toCamel, но с capNext = false
	s = strings.Trim(s, " ")
	result := strings.Builder{}
	capNext := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			result.WriteRune(r)
			capNext = false
		case r >= '0' && r <= '9':
			result.WriteRune(r)
			capNext = false
		case r >= 'a' && r <= 'z':
			if capNext {
				result.WriteRune(unicode.ToUpper(r))
			} else {
				result.WriteRune(r)
			}
			capNext = false
		case r == '_' || r == ' ' || r == '-' || r == '.':
			capNext = true
		default:
			result.WriteRune(r)
			capNext = false
		}
	}
	return result.String()
}

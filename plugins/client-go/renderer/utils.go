// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/core"
)

const (
	_ctx_  = "ctx"
	_ftx_  = "ftx"
	_next_ = "next"
)

type key string

const keyCode key = "code"
const keyPackage key = "package"

var numberSequence = regexp.MustCompile(`([a-zA-Z])(\d+)([a-zA-Z]?)`)
var numberReplacement = []byte(`$1 $2 $3`)

// toCamelInitCase конвертирует строку в CamelCase.
func toCamelInitCase(s string, initCase bool) string {
	s = addWordBoundariesToNumbers(s)
	s = strings.Trim(s, " ")
	n := ""
	capNext := initCase
	for _, v := range s {
		if v >= 'A' && v <= 'Z' {
			n += string(v)
		}
		if v >= '0' && v <= '9' {
			n += string(v)
		}
		if v >= 'a' && v <= 'z' {
			if capNext {
				n += strings.ToUpper(string(v))
			} else {
				n += string(v)
			}
		}
		if v == '_' || v == ' ' || v == '-' {
			capNext = true
		} else {
			capNext = false
		}
	}
	return n
}

// ToCamel конвертирует строку в CamelCase.
func ToCamel(s string) string {
	return toCamelInitCase(s, true)
}

func isAllUpper(s string) bool {
	for _, v := range s {
		if v >= 'a' && v <= 'z' {
			return false
		}
	}
	return true
}

// ToLowerCamel конвертирует строку в lowerCamelCase.
func ToLowerCamel(s string) string {
	if isAllUpper(s) {
		return s
	}
	if s == "" {
		return s
	}
	if r := rune(s[0]); r >= 'A' && r <= 'Z' {
		s = strings.ToLower(string(r)) + s[1:]
	}
	return toCamelInitCase(s, false)
}

func addWordBoundariesToNumbers(s string) string {
	b := []byte(s)
	b = numberSequence.ReplaceAll(b, numberReplacement)
	return string(b)
}

// fieldTypeFromVariable конвертирует тип из Variable в код jennifer.
// Использует информацию о массивах/map из Variable.
func (r *ClientRenderer) fieldTypeFromVariable(ctx context.Context, variable *core.Variable, allowEllipsis bool) *Statement {
	c := &Statement{}

	// Добавляем указатели
	for i := 0; i < variable.NumberOfPointers; i++ {
		c.Op("*")
	}

	// Обрабатываем ellipsis
	if variable.IsEllipsis && allowEllipsis {
		c.Op("...")
		if variable.TypeID != "" {
			return c.Add(r.fieldType(ctx, variable.TypeID, 0, false))
		}
		return c
	}

	// Обрабатываем массивы и слайсы
	if variable.IsSlice || variable.ArrayLen > 0 {
		// ВАЖНО: если TypeID указывает на именованный тип из внешнего пакета (например, uuid.UUID),
		// используем его как именованный тип, а не как массив [16]byte
		if variable.TypeID != "" {
			typ, ok := r.project.Types[variable.TypeID]
			if ok && typ.ImportPkgPath != "" && typ.TypeName != "" {
				if !r.isTypeFromCurrentProject(typ.ImportPkgPath) {
					// Это именованный тип из внешнего пакета - используем его как есть
					// Добавляем указатели на переменной (если есть *uuid.UUID)
					for i := 0; i < variable.NumberOfPointers; i++ {
						c.Op("*")
					}
					// Используем именованный тип из внешнего пакета
					if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
						packageName := typ.PkgName
						if packageName == "" {
							packageName = filepath.Base(typ.ImportPkgPath)
						}
						// Если ImportAlias установлен и отличается от PkgName, используем алиас
						if typ.ImportAlias != "" && typ.ImportAlias != packageName {
							srcFile.ImportName(typ.ImportPkgPath, typ.ImportAlias)
							return c.Qual(typ.ImportPkgPath, typ.TypeName)
						}
						// Иначе используем реальное имя пакета
						srcFile.ImportName(typ.ImportPkgPath, packageName)
						return c.Qual(typ.ImportPkgPath, typ.TypeName)
					}
					return c.Qual(typ.ImportPkgPath, typ.TypeName)
				}
			}
		}
		// Сначала добавляем указатели на переменной (если есть *[]T)
		for i := 0; i < variable.NumberOfPointers; i++ {
			c.Op("*")
		}
		// Затем добавляем массив/слайс
		if variable.IsSlice {
			c.Index()
		} else {
			c.Index(Lit(variable.ArrayLen))
		}
		// Тип элемента - применяем указатели на элементе к типу элемента
		if variable.TypeID != "" {
			return c.Add(r.fieldType(ctx, variable.TypeID, variable.ElementPointers, false))
		}
		// Если TypeID пустой, используем any как fallback
		return c.Add(Id("any"))
	}

	// Обрабатываем map
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		keyType := r.fieldType(ctx, variable.MapKeyID, variable.MapKeyPointers, false)
		valueType := r.fieldType(ctx, variable.MapValueID, variable.ElementPointers, false)
		return c.Map(keyType).Add(valueType)
	}

	// Базовый тип
	return c.Add(r.fieldType(ctx, variable.TypeID, 0, false))
}

// fieldType конвертирует тип из renderer в код jennifer.
func (r *ClientRenderer) fieldType(ctx context.Context, typeID string, numberOfPointers int, allowEllipsis bool) *Statement {
	c := &Statement{}

	// Добавляем указатели
	for i := 0; i < numberOfPointers; i++ {
		c.Op("*")
	}

	// Получаем тип из проекта
	typ, ok := r.project.Types[typeID]
	if !ok {
		// Тип не найден в project.Types - извлекаем информацию из typeID
		// Формат typeID: "pkgPath:TypeName" для импортированных типов, или просто "TypeName" для встроенных
		if strings.Contains(typeID, ":") {
			parts := strings.SplitN(typeID, ":", 2)
			if len(parts) == 2 && parts[1] != "" {
				pkgPath := parts[0]
				typeName := parts[1]
				// Импортированный тип
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					baseName := filepath.Base(pkgPath)
					switch {
					case strings.HasSuffix(pkgPath, baseName):
						srcFile.ImportName(pkgPath, baseName)
					default:
						srcFile.ImportName(pkgPath, baseName)
					}
				}
				return c.Qual(pkgPath, typeName)
			}
		}
		// Встроенный тип - просто имя
		return c.Id(typeID)
	}

	// ВАЖНО: для типов из внешних пакетов (не из текущего проекта) используем их как именованные типы,
	// независимо от Kind. Например, uuid.UUID имеет Kind == TypeKindArray, но это именованный тип
	// из внешнего пакета, и его нужно использовать как uuid.UUID, а не как [16]byte
	// Эта проверка должна быть ПЕРВОЙ, до всех остальных обработок (ellipsis, switch по Kind и т.д.)
	if typ.ImportPkgPath != "" && typ.TypeName != "" {
		if !r.isTypeFromCurrentProject(typ.ImportPkgPath) {
			// Тип из внешнего пакета - используем информацию из renderer напрямую
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				// PkgName содержит реальное имя пакета из package декларации (например, "uuid")
				packageName := typ.PkgName
				if packageName == "" {
					// Fallback на последнюю часть пути, если PkgName не установлен
					packageName = filepath.Base(typ.ImportPkgPath)
				}
				// Если ImportAlias установлен и отличается от PkgName, используем алиас
				if typ.ImportAlias != "" && typ.ImportAlias != packageName {
					srcFile.ImportName(typ.ImportPkgPath, typ.ImportAlias)
					return c.Qual(typ.ImportPkgPath, typ.TypeName)
				}
				// Иначе используем реальное имя пакета
				srcFile.ImportName(typ.ImportPkgPath, packageName)
				return c.Qual(typ.ImportPkgPath, typ.TypeName)
			}
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
	}

	// Обрабатываем ellipsis
	if typ.IsEllipsis && allowEllipsis {
		c.Op("...")
		if typ.ArrayOfID != "" {
			return c.Add(r.fieldType(ctx, typ.ArrayOfID, 0, false))
		}
		return c
	}

	// Обрабатываем в зависимости от вида типа
	switch typ.Kind {
	case core.TypeKindArray:
		switch {
		case typ.IsSlice:
			c.Index()
		case typ.ArrayLen > 0:
			c.Index(Lit(typ.ArrayLen))
		default:
			c.Index()
		}
		if typ.ArrayOfID != "" {
			return c.Add(r.fieldType(ctx, typ.ArrayOfID, 0, false))
		}
		return c

	case core.TypeKindMap:
		if typ.MapKeyID != "" && typ.MapValueID != "" {
			keyType := r.fieldType(ctx, typ.MapKeyID, 0, false)
			valueType := r.fieldType(ctx, typ.MapValueID, 0, false)
			return c.Map(keyType).Add(valueType)
		}
		return c

	case core.TypeKindChan:
		chanType := c
		switch typ.ChanDirection {
		case 1: // send only
			chanType = chanType.Chan().Op("<-")
		case 2: // receive only
			chanType = chanType.Op("<-").Chan()
		default: // both
			chanType = chanType.Chan()
		}
		if typ.ChanOfID != "" {
			return chanType.Add(r.fieldType(ctx, typ.ChanOfID, 0, false))
		}
		return chanType

	case core.TypeKindStruct:
		// ВАЖНО: все типы из текущего проекта должны генерироваться локально и использоваться из dto пакета
		if typ.ImportPkgPath != "" && typ.TypeName != "" {
			// Проверяем, является ли тип из текущего проекта
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта - используем dto пакет
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
			// Тип из внешнего пакета - импортируем как обычно
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				// Используем PkgName (реальное имя пакета) для ImportName
				packageName := typ.PkgName
				if packageName == "" {
					packageName = filepath.Base(typ.ImportPkgPath)
				}
				// Если ImportAlias установлен и отличается от PkgName, используем алиас
				if typ.ImportAlias != "" && typ.ImportAlias != packageName {
					srcFile.ImportName(typ.ImportPkgPath, typ.ImportAlias)
					return c.Qual(typ.ImportPkgPath, typ.TypeName)
				}
				// Иначе используем реальное имя пакета
				srcFile.ImportName(typ.ImportPkgPath, packageName)
				return c.Qual(typ.ImportPkgPath, typ.TypeName)
			}
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
		return c.Id(typ.TypeName)

	case core.TypeKindInterface:
		// ВАЖНО: все типы из текущего проекта должны генерироваться локально и использоваться из dto пакета
		if typ.ImportPkgPath != "" && typ.TypeName != "" {
			// Проверяем, является ли тип из текущего проекта
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта - используем dto пакет
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
			// Тип из внешнего пакета - импортируем как обычно
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				// Используем PkgName (реальное имя пакета) для ImportName
				packageName := typ.PkgName
				if packageName == "" {
					packageName = filepath.Base(typ.ImportPkgPath)
				}
				// Если ImportAlias установлен и отличается от PkgName, используем алиас
				if typ.ImportAlias != "" && typ.ImportAlias != packageName {
					srcFile.ImportName(typ.ImportPkgPath, typ.ImportAlias)
					return c.Qual(typ.ImportPkgPath, typ.TypeName)
				}
				// Иначе используем реальное имя пакета
				srcFile.ImportName(typ.ImportPkgPath, packageName)
				return c.Qual(typ.ImportPkgPath, typ.TypeName)
			}
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
		return c.Id(typ.TypeName)

	case core.TypeKindFunction:
		args := make([]Code, 0, len(typ.FunctionArgs))
		for _, arg := range typ.FunctionArgs {
			argType := r.fieldTypeFromVariable(ctx, arg, false)
			args = append(args, argType)
		}
		results := make([]Code, 0, len(typ.FunctionResults))
		for _, res := range typ.FunctionResults {
			resType := r.fieldTypeFromVariable(ctx, res, false)
			results = append(results, resType)
		}
		return c.Func().Params(args...).Params(results...)

	// Базовые типы Go (string, int, int64, и т.д.)
	case core.TypeKindAlias:
		// Алиас - получаем базовый тип через AliasOf
		if typ.AliasOf != "" {
			return r.fieldType(ctx, typ.AliasOf, numberOfPointers, allowEllipsis)
		}
		// Если AliasOf пустой, fallback на базовый тип
		return c.Id(typeID)

	case core.TypeKindString, core.TypeKindInt, core.TypeKindInt8, core.TypeKindInt16,
		core.TypeKindInt32, core.TypeKindInt64, core.TypeKindUint, core.TypeKindUint8,
		core.TypeKindUint16, core.TypeKindUint32, core.TypeKindUint64,
		core.TypeKindFloat32, core.TypeKindFloat64, core.TypeKindBool,
		core.TypeKindByte, core.TypeKindRune, core.TypeKindError, core.TypeKindAny:
		// ВАЖНО: все типы из текущего проекта должны генерироваться локально и использоваться из dto пакета
		// Если у типа есть ImportPkgPath и TypeName, это именованный тип (например, UserID int64, Email string)
		if typ.ImportPkgPath != "" && typ.TypeName != "" {
			// Проверяем, является ли тип из текущего проекта
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта - используем dto пакет
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
			// Тип не был собран - это стандартная библиотека или внешний пакет, импортируем
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				baseName := filepath.Base(typ.ImportPkgPath)
				switch {
				case typ.ImportAlias != "" && typ.ImportAlias != baseName:
					srcFile.ImportName(typ.ImportPkgPath, typ.ImportAlias)
				case strings.HasSuffix(typ.ImportPkgPath, baseName):
					srcFile.ImportName(typ.ImportPkgPath, baseName)
				default:
					srcFile.ImportName(typ.ImportPkgPath, baseName)
				}
				return c.Qual(typ.ImportPkgPath, typ.TypeName)
			}
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
		// Встроенный базовый тип - используем Kind как имя типа
		return c.Id(string(typ.Kind))

	default:
		return c
	}
}

func (r *ClientRenderer) funcDefinitionParams(ctx context.Context, vars []*core.Variable) *Statement {
	c := &Statement{}
	c.ListFunc(func(gr *Group) {
		for _, v := range vars {
			typeCode := r.fieldTypeFromVariable(ctx, v, true)
			varName := v.Name
			if varName == "" {
				varName = "_"
			} else {
				varName = ToLowerCamel(varName)
			}
			gr.Id(varName).Add(typeCode)
		}
	})
	return c
}

// isContextFirst проверяет, является ли первый аргумент context.Context.
func (r *ClientRenderer) isContextFirst(vars []*core.Variable) bool {

	if len(vars) == 0 {
		return false
	}
	typ, ok := r.project.Types[vars[0].TypeID]
	if !ok {
		// Проверяем по typeID - если это "context:Context", то это context.Context
		return vars[0].TypeID == "context:Context"
	}
	return typ.Kind == core.TypeKindInterface && typ.ImportPkgPath == "context" && typ.TypeName == "Context"
}

// isErrorLast проверяет, является ли последний результат error.
func (r *ClientRenderer) isErrorLast(vars []*core.Variable) bool {

	if len(vars) == 0 {
		return false
	}
	// Проверяем по typeID - если это "error", то это встроенный тип error
	return vars[len(vars)-1].TypeID == "error"
}

// argsWithoutContext возвращает аргументы без первого context.Context, если он есть.
func (r *ClientRenderer) argsWithoutContext(method *core.Method) []*core.Variable {

	if r.isContextFirst(method.Args) {
		return method.Args[1:]
	}
	return method.Args
}

// resultsWithoutError возвращает результаты без последнего error, если он есть.
func (r *ClientRenderer) resultsWithoutError(method *core.Method) []*core.Variable {

	if r.isErrorLast(method.Results) {
		return method.Results[:len(method.Results)-1]
	}
	return method.Results
}

// requestStructName возвращает имя структуры request для метода.
func (r *ClientRenderer) requestStructName(contract *core.Contract, method *core.Method) string {

	return "request" + contract.Name + method.Name
}

// responseStructName возвращает имя структуры response для метода.
func (r *ClientRenderer) responseStructName(contract *core.Contract, method *core.Method) string {

	return "response" + contract.Name + method.Name
}

// contractKeys возвращает отсортированный список имён контрактов.
func (r *ClientRenderer) contractKeys() []string {
	keys := make([]string, 0, len(r.project.Contracts))
	for _, contract := range r.project.Contracts {
		keys = append(keys, contract.Name)
	}
	// Сортируем для детерминированного порядка
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// methodIsJsonRPC проверяет, является ли метод JSON-RPC методом.
func (r *ClientRenderer) methodIsJsonRPC(contract *core.Contract, method *core.Method) bool {
	if method == nil || method.Annotations == nil {
		return false
	}
	// Метод является JSON-RPC, если контракт имеет jsonRPC-server и метод НЕ имеет http-method
	return contract != nil && r.contains(contract.Annotations, TagServerJsonRPC) && !r.contains(method.Annotations, TagMethodHTTP)
}

// methodIsHTTP проверяет, является ли метод HTTP методом.
func (r *ClientRenderer) methodIsHTTP(method *core.Method) bool {
	if method == nil || method.Annotations == nil {
		return false
	}
	return r.contains(method.Annotations, TagMethodHTTP)
}

// isTypeFromCurrentProject проверяет, является ли тип из текущего проекта.
func (r *ClientRenderer) isTypeFromCurrentProject(importPkgPath string) bool {
	// Если ImportPkgPath начинается с ModulePath проекта, это тип из текущего проекта
	if r.project.ModulePath != "" && strings.HasPrefix(importPkgPath, r.project.ModulePath) {
		return true
	}
	return false
}

// parseTagsFromDocs парсит аннотации из документации
func (r *ClientRenderer) parseTagsFromDocs(docs string) map[string]string {
	tags := make(map[string]string)
	if docs == "" {
		return tags
	}

	lines := strings.Split(docs, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@") {
			// Убираем @ в начале
			line = strings.TrimPrefix(line, "@")
			// Ищем первое двоеточие или пробел
			idx := strings.IndexAny(line, ": ")
			if idx > 0 {
				key := strings.TrimSpace(line[:idx])
				value := strings.TrimSpace(line[idx+1:])
				// Убираем @ в начале значения, если есть
				value = strings.TrimPrefix(value, "@")
				tags[key] = value
			} else {
				// Ключ без значения
				tags[line] = ""
			}
		}
	}
	return tags
}

// generateExampleValueFromVariable генерирует пример значения для переменной
func (r *ClientRenderer) generateExampleValueFromVariable(variable *core.Variable, docs, pkgPath string) string {
	// Обрабатываем массивы и слайсы
	if variable.IsSlice || variable.ArrayLen > 0 {
		elemType := r.goTypeString(variable.TypeID, pkgPath)
		if variable.IsSlice {
			return fmt.Sprintf("[]%s{}", elemType)
		}
		return fmt.Sprintf("[%d]%s{}", variable.ArrayLen, elemType)
	}

	// Обрабатываем map
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		keyType := r.goTypeString(variable.MapKeyID, pkgPath)
		valueType := r.goTypeString(variable.MapValueID, pkgPath)
		return fmt.Sprintf("map[%s]%s{}", keyType, valueType)
	}

	// Базовый тип - используем простые примеры
	typeStr := r.goTypeString(variable.TypeID, pkgPath)
	switch typeStr {
	case "string":
		return `"example"`
	case "int", "int8", "int16", "int32", "int64":
		return "0"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "0"
	case "float32", "float64":
		return "0.0"
	case "bool":
		return "false"
	default:
		// Для сложных типов возвращаем пустую структуру или nil
		if variable.NumberOfPointers > 0 {
			return "nil"
		}
		return fmt.Sprintf("%s{}", typeStr)
	}
}

// exchangeField представляет поле для обмена данными.
type exchangeField struct {
	name             string
	typeID           string
	numberOfPointers int
	isSlice          bool
	arrayLen         int
	isEllipsis       bool
	elementPointers  int
	mapKeyID         string
	mapValueID       string
	tags             map[string]string
}

// fieldsArgument возвращает поля для аргументов метода.
func (r *ClientRenderer) fieldsArgument(method *core.Method) []exchangeField {
	vars := r.argsWithoutContext(method)
	return r.varsToFields(vars, method.Annotations)
}

// fieldsResult возвращает поля для результатов метода.
func (r *ClientRenderer) fieldsResult(method *core.Method) []exchangeField {
	vars := r.resultsWithoutError(method)
	return r.varsToFields(vars, method.Annotations)
}

// varsToFields конвертирует переменные в поля для обмена данными.
func (r *ClientRenderer) varsToFields(vars []*core.Variable, methodTags map[string]string) []exchangeField {
	fields := make([]exchangeField, 0, len(vars))
	for _, v := range vars {
		field := exchangeField{
			name:             v.Name,
			typeID:           v.TypeID,
			numberOfPointers: v.NumberOfPointers,
			isSlice:          v.IsSlice,
			arrayLen:         v.ArrayLen,
			isEllipsis:       v.IsEllipsis,
			elementPointers:  v.ElementPointers,
			mapKeyID:         v.MapKeyID,
			mapValueID:       v.MapValueID,
			tags:             make(map[string]string),
		}
		// Обрабатываем теги из аннотаций метода
		// Формат ключа: "tag:{variableName}:{tagName}"
		prefix := fmt.Sprintf("tag:%s:", v.Name)
		for key, value := range methodTags {
			if strings.HasPrefix(key, prefix) {
				tagName := strings.TrimPrefix(key, prefix)
				if tagName == "tag" {
					// Формат: tag:json:fieldName,omitempty|tag:xml:fieldName
					if list := strings.Split(value, "|"); len(list) > 0 {
						for _, item := range list {
							if tokens := strings.Split(item, ":"); len(tokens) >= 2 {
								tagName := tokens[0]
								tagValue := strings.Join(tokens[1:], ":")
								if tagValue == "inline" {
									tagValue = ",inline"
								}
								field.tags[tagName] = tagValue
							}
						}
					}
				} else {
					field.tags[tagName] = value
				}
			}
		}
		fields = append(fields, field)
	}
	return fields
}

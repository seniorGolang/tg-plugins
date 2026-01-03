// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
	"tgp/plugins/server/renderer/types"
)

// varHeaderMap возвращает маппинг переменных на HTTP заголовки.
func (r *contractRenderer) varHeaderMap(method *core.Method) map[string]string {

	headers := make(map[string]string)
	if httpHeaders := method.Annotations.Value(TagHttpHeader, ""); httpHeaders != "" {
		headerPairs := strings.Split(httpHeaders, ",")
		for _, pair := range headerPairs {
			if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
				arg := strings.TrimSpace(pairTokens[0])
				header := strings.TrimSpace(pairTokens[1])
				headers[arg] = header
			}
		}
	}
	return headers
}

// varCookieMap возвращает маппинг переменных на HTTP cookies.
func (r *contractRenderer) varCookieMap(method *core.Method) map[string]string {

	cookies := make(map[string]string)
	if httpCookies := method.Annotations.Value(TagHttpCookies, ""); httpCookies != "" {
		cookiePairs := strings.Split(httpCookies, ",")
		for _, pair := range cookiePairs {
			if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
				arg := strings.TrimSpace(pairTokens[0])
				cookie := strings.TrimSpace(pairTokens[1])
				cookies[arg] = cookie
			}
		}
	}
	return cookies
}

// argPathMap возвращает маппинг аргументов на path параметры.
func (r *contractRenderer) argPathMap(method *core.Method) map[string]string {

	paths := make(map[string]string)
	if urlPath := method.Annotations.Value(TagHttpPath, ""); urlPath != "" {
		urlTokens := strings.Split(urlPath, "/")
		for _, token := range urlTokens {
			if strings.HasPrefix(token, ":") {
				arg := strings.TrimSpace(strings.TrimPrefix(token, ":"))
				paths[arg] = arg
			}
		}
	}
	return paths
}

// argParamMap возвращает маппинг аргументов на query параметры.
func (r *contractRenderer) argParamMap(method *core.Method) map[string]string {

	params := make(map[string]string)
	if urlArgs := method.Annotations.Value(TagHttpArg, ""); urlArgs != "" {
		paramPairs := strings.Split(urlArgs, ",")
		for _, pair := range paramPairs {
			if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
				arg := strings.TrimSpace(pairTokens[0])
				param := strings.TrimSpace(pairTokens[1])
				params[arg] = param
			}
		}
	}
	return params
}

// argByName находит аргумент по имени.
func (r *contractRenderer) argByName(method *core.Method, argName string) *core.Variable {

	argName = strings.TrimPrefix(argName, "!")
	for _, arg := range method.Args {
		if arg.Name == argName {
			return arg
		}
	}
	return nil
}

// resultByName находит результат по имени.
func (r *contractRenderer) resultByName(method *core.Method, retName string) *core.Variable {

	for _, ret := range method.Results {
		if ret.Name == retName {
			return ret
		}
	}
	return nil
}

// retCookieMap возвращает маппинг результатов на cookies.
func (r *contractRenderer) retCookieMap(method *core.Method) map[string]string {

	cookies := make(map[string]string)
	cookieMap := r.varCookieMap(method)
	for varName, cookieName := range cookieMap {
		if r.resultByName(method, varName) != nil {
			cookies[varName] = cookieName
		}
	}
	return cookies
}

// argsWithoutSpecialArgs возвращает аргументы без path, query, header, cookie параметров.
func (r *contractRenderer) argsWithoutSpecialArgs(method *core.Method) []*core.Variable {

	vars := make([]*core.Variable, 0)
	argsAll := argsWithoutContext(method)
	pathMap := r.argPathMap(method)
	paramMap := r.argParamMap(method)
	headerMap := r.varHeaderMap(method)
	cookieMap := r.varCookieMap(method)

	for _, arg := range argsAll {
		_, inPath := pathMap[arg.Name]
		_, inArgs := paramMap[arg.Name]
		_, inHeader := headerMap[arg.Name]
		_, inCookie := cookieMap[arg.Name]

		if !inArgs && !inPath && !inHeader && !inCookie {
			vars = append(vars, arg)
		}
	}
	return vars
}

// isBuiltinTypeID проверяет, является ли TypeID встроенным типом Go.
func isBuiltinTypeID(typeID string) bool {

	switch typeID {
	case "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"bool", "byte", "rune", "error", "any":
		return true
	}
	return false
}

// argFromString генерирует код для извлечения аргумента из строки (path, query, header, cookie).
func (r *contractRenderer) argFromString(srcFile *GoFile, typeGen *types.Generator, method *core.Method, typeName string, varMap map[string]string, srcCode func(srcName string) Code, errStatement func(arg, header string) *Statement) *Statement {

	block := Line()
	if len(varMap) != 0 {
		for fullArgName, srcName := range varMap {
			fullArgName = strings.TrimPrefix(fullArgName, "!")
			argTokens := strings.Split(fullArgName, ".")
			argName := argTokens[0]
			argVarName := strings.Join(argTokens, "")
			arg := r.argByName(method, argName)
			if arg == nil {
				continue
			}
			srcName = strings.TrimPrefix(srcName, "!")

			// Определяем тип для генерации кода, используя данные из core напрямую
			var argTypeName string
			var typ *core.Type
			var fieldTypeID string
			var fieldIsSlice bool
			var fieldElementPointers int
			var fieldNumberOfPointers int
			var fieldMapKeyID string
			var fieldMapValueID string
			var fieldMapKeyPointers int

			// Если это вложенное поле (например, "data.name"), используем данные из StructField напрямую
			if len(argTokens) > 1 {
				argType, ok := r.project.Types[arg.TypeID]
				if !ok {
					continue
				}
				if argType.Kind == core.TypeKindAlias && argType.AliasOf != "" {
					if baseType, ok := r.project.Types[argType.AliasOf]; ok {
						argType = baseType
					}
				}
				currentType := argType
				var field *core.StructField
				for i := 1; i < len(argTokens); i++ {
					for _, f := range currentType.StructFields {
						if f.Name == argTokens[i] {
							field = f
							break
						}
					}
					if field == nil {
						break
					}
					// Если это не последний элемент пути, переходим к следующему типу
					if i+1 < len(argTokens) {
						nextType, ok := r.project.Types[field.TypeID]
						if !ok {
							field = nil
							break
						}
						if nextType.Kind == core.TypeKindAlias && nextType.AliasOf != "" {
							if baseType, ok := r.project.Types[nextType.AliasOf]; ok {
								currentType = baseType
							} else {
								field = nil
								break
							}
						} else {
							currentType = nextType
						}
					}
				}
				if field == nil {
					continue
				}
				fieldTypeID = field.TypeID
				fieldIsSlice = field.IsSlice
				fieldElementPointers = field.ElementPointers
				fieldNumberOfPointers = field.NumberOfPointers
				fieldMapKeyID = field.MapKeyID
				fieldMapValueID = field.MapValueID
				fieldMapKeyPointers = field.MapKeyPointers
			} else {
				fieldTypeID = arg.TypeID
				fieldIsSlice = arg.IsSlice
				fieldElementPointers = arg.ElementPointers
				fieldNumberOfPointers = arg.NumberOfPointers
				fieldMapKeyID = arg.MapKeyID
				fieldMapValueID = arg.MapValueID
				fieldMapKeyPointers = arg.MapKeyPointers
			}

			switch {
			case fieldIsSlice:
				// Для слайсов TypeID уже содержит тип элемента без префикса []
				elementTypeID := fieldTypeID
				if isBuiltinTypeID(elementTypeID) {
					argTypeName = "[]" + elementTypeID
				} else {
					var ok bool
					typ, ok = r.project.Types[elementTypeID]
					if !ok {
						continue
					}
					argTypeName = "[]" + typ.TypeName
					if typ.TypeName == "" {
						argTypeName = "[]" + string(typ.Kind)
					}
				}
			case isBuiltinTypeID(fieldTypeID):
				argTypeName = fieldTypeID
			default:
				var ok bool
				typ, ok = r.project.Types[fieldTypeID]
				if !ok {
					continue
				}
				argTypeName = typ.TypeName
				if argTypeName == "" {
					argTypeName = string(typ.Kind)
				}
			}

			argID := Id(argVarName)

			// Обработка указателей (если нужно)
			if fieldNumberOfPointers > 0 {
				argID = Op("&").Add(argID)
			}

			// Всегда используем проверку на пустоту, как в эталонной реализации
			block.If(Id("_" + argVarName).Op(":=").Add(srcCode(srcName)).Op(";").Id("_" + argVarName).Op("!=").Lit("")).
				BlockFunc(func(bg *Group) {
					// Объявляем переменную нужного типа
					if typ != nil && typ.ImportPkgPath != "" {
						// Если тип импортирован, используем полное имя (может содержать точку)
						typeParts := strings.Split(argTypeName, ".")
						if len(typeParts) > 1 {
							// Полное имя типа (например, "uuid.UUID")
							bg.Var().Id(argVarName).Qual(typ.ImportPkgPath, typeParts[1])
						} else {
							bg.Var().Id(argVarName).Qual(typ.ImportPkgPath, argTypeName)
						}
					} else {
						// Встроенный или локальный тип
						bg.Var().Id(argVarName).Id(argTypeName)
					}

					// Конвертируем строку в нужный тип, создавая временный Variable из данных поля
					fieldVar := &core.Variable{
						TypeID:           fieldTypeID,
						IsSlice:          fieldIsSlice,
						ElementPointers:  fieldElementPointers,
						NumberOfPointers: fieldNumberOfPointers,
						MapKeyID:         fieldMapKeyID,
						MapValueID:       fieldMapValueID,
						MapKeyPointers:   fieldMapKeyPointers,
					}
					bg.Add(r.argToTypeConverter(srcFile, typeGen, Id("_"+argVarName), fieldVar, Id(argVarName), errStatement(argVarName, srcName)))

					// Присваиваем значение в request
					reqID := bg.Id("request").Dot(toCamel(argName))
					if len(argTokens) > 1 {
						for _, token := range argTokens[1:] {
							reqID = reqID.Dot(toCamel(token))
						}
						reqID.Op("=").Add(argID)
					} else {
						reqID.Op("=").Add(argID)
					}
				}).Line()
		}
	}
	return block
}

// argFromStringOrdered генерирует код для извлечения аргументов из строки с сохранением порядка.
func (r *contractRenderer) argFromStringOrdered(srcFile *GoFile, typeGen *types.Generator, method *core.Method, typeName string, varMap map[string]string, orderedArgs []string, srcCode func(srcName string) Code, errStatement func(arg, header string) *Statement) *Statement {

	block := Line()
	if len(varMap) != 0 {
		// Используем упорядоченный список, если он предоставлен
		var argsToProcess []string
		if len(orderedArgs) > 0 {
			argsToProcess = orderedArgs
		} else {
			// Если порядок не указан, используем порядок из map (не гарантирован)
			for argName := range varMap {
				argsToProcess = append(argsToProcess, argName)
			}
		}
		for _, fullArgName := range argsToProcess {
			srcName, ok := varMap[fullArgName]
			if !ok {
				continue
			}
			fullArgName = strings.TrimPrefix(fullArgName, "!")
			argTokens := strings.Split(fullArgName, ".")
			argName := argTokens[0]
			argVarName := strings.Join(argTokens, "")
			arg := r.argByName(method, argName)
			if arg == nil {
				continue
			}
			srcName = strings.TrimPrefix(srcName, "!")

			// Определяем тип для генерации кода, используя данные из core напрямую
			var argTypeName string
			var typ *core.Type
			var fieldTypeID string
			var fieldIsSlice bool
			var fieldElementPointers int
			var fieldNumberOfPointers int
			var fieldMapKeyID string
			var fieldMapValueID string
			var fieldMapKeyPointers int

			// Если это вложенное поле (например, "data.name"), используем данные из StructField напрямую
			if len(argTokens) > 1 {
				argType, ok := r.project.Types[arg.TypeID]
				if !ok {
					continue
				}
				if argType.Kind == core.TypeKindAlias && argType.AliasOf != "" {
					if baseType, ok := r.project.Types[argType.AliasOf]; ok {
						argType = baseType
					}
				}
				currentType := argType
				var field *core.StructField
				for i := 1; i < len(argTokens); i++ {
					for _, f := range currentType.StructFields {
						if f.Name == argTokens[i] {
							field = f
							break
						}
					}
					if field == nil {
						break
					}
					// Если это не последний элемент пути, переходим к следующему типу
					if i+1 < len(argTokens) {
						nextType, ok := r.project.Types[field.TypeID]
						if !ok {
							field = nil
							break
						}
						if nextType.Kind == core.TypeKindAlias && nextType.AliasOf != "" {
							if baseType, ok := r.project.Types[nextType.AliasOf]; ok {
								currentType = baseType
							} else {
								field = nil
								break
							}
						} else {
							currentType = nextType
						}
					}
				}
				if field == nil {
					continue
				}
				fieldTypeID = field.TypeID
				fieldIsSlice = field.IsSlice
				fieldElementPointers = field.ElementPointers
				fieldNumberOfPointers = field.NumberOfPointers
				fieldMapKeyID = field.MapKeyID
				fieldMapValueID = field.MapValueID
				fieldMapKeyPointers = field.MapKeyPointers
			} else {
				fieldTypeID = arg.TypeID
				fieldIsSlice = arg.IsSlice
				fieldElementPointers = arg.ElementPointers
				fieldNumberOfPointers = arg.NumberOfPointers
				fieldMapKeyID = arg.MapKeyID
				fieldMapValueID = arg.MapValueID
				fieldMapKeyPointers = arg.MapKeyPointers
			}

			switch {
			case fieldIsSlice:
				// Для слайсов TypeID уже содержит тип элемента без префикса []
				elementTypeID := fieldTypeID
				if isBuiltinTypeID(elementTypeID) {
					argTypeName = "[]" + elementTypeID
				} else {
					var ok bool
					typ, ok = r.project.Types[elementTypeID]
					if !ok {
						continue
					}
					argTypeName = "[]" + typ.TypeName
					if typ.TypeName == "" {
						argTypeName = "[]" + string(typ.Kind)
					}
				}
			case isBuiltinTypeID(fieldTypeID):
				argTypeName = fieldTypeID
			default:
				var ok bool
				typ, ok = r.project.Types[fieldTypeID]
				if !ok {
					continue
				}
				argTypeName = typ.TypeName
				if argTypeName == "" {
					argTypeName = string(typ.Kind)
				}
			}

			argID := Id(argVarName)

			// Обработка указателей (если нужно)
			if fieldNumberOfPointers > 0 {
				argID = Op("&").Add(argID)
			}

			// Всегда используем проверку на пустоту, как в эталонной реализации
			block.If(Id("_" + argVarName).Op(":=").Add(srcCode(srcName)).Op(";").Id("_" + argVarName).Op("!=").Lit("")).
				BlockFunc(func(bg *Group) {
					// Объявляем переменную нужного типа
					if typ != nil && typ.ImportPkgPath != "" {
						// Если тип импортирован, используем полное имя (может содержать точку)
						typeParts := strings.Split(argTypeName, ".")
						if len(typeParts) > 1 {
							// Полное имя типа (например, "uuid.UUID")
							bg.Var().Id(argVarName).Qual(typ.ImportPkgPath, typeParts[1])
						} else {
							bg.Var().Id(argVarName).Qual(typ.ImportPkgPath, argTypeName)
						}
					} else {
						// Встроенный или локальный тип
						bg.Var().Id(argVarName).Id(argTypeName)
					}

					// Конвертируем строку в нужный тип, создавая временный Variable из данных поля
					fieldVar := &core.Variable{
						TypeID:           fieldTypeID,
						IsSlice:          fieldIsSlice,
						ElementPointers:  fieldElementPointers,
						NumberOfPointers: fieldNumberOfPointers,
						MapKeyID:         fieldMapKeyID,
						MapValueID:       fieldMapValueID,
						MapKeyPointers:   fieldMapKeyPointers,
					}
					bg.Add(r.argToTypeConverter(srcFile, typeGen, Id("_"+argVarName), fieldVar, Id(argVarName), errStatement(argVarName, srcName)))

					// Присваиваем значение в request
					reqID := bg.Id("request").Dot(toCamel(argName))
					if len(argTokens) > 1 {
						for _, token := range argTokens[1:] {
							reqID = reqID.Dot(toCamel(token))
						}
						reqID.Op("=").Add(argID)
					} else {
						reqID.Op("=").Add(argID)
					}
				}).Line()
		}
	}
	return block
}

// argToTypeConverter генерирует код для конвертации строки в нужный тип.
func (r *contractRenderer) argToTypeConverter(srcFile *GoFile, typeGen *types.Generator, from *Statement, arg *core.Variable, id *Statement, errStatement *Statement) *Statement {

	op := "="

	// Обработка слайсов
	if arg.IsSlice {
		// Для слайсов парсим через strings.Split и конвертируем каждый элемент
		// TypeID уже содержит тип элемента без префикса []
		elementTypeID := arg.TypeID
		// Создаем переменную для элемента
		elementVar := &core.Variable{
			Name:             "elem",
			TypeID:           elementTypeID,
			NumberOfPointers: arg.ElementPointers,
			IsSlice:          false,
		}
		// Генерируем код для парсинга слайса
		srcFile.ImportName(PackageStrings, "strings")
		// Получаем тип элемента через typeGen для правильной обработки пакетов
		elementTypeCode := typeGen.FieldType(elementTypeID, arg.ElementPointers, false)
		return BlockFunc(func(bg *Group) {
			bg.Id("parts").Op(":=").Qual(PackageStrings, "Split").Call(from, Lit(","))
			bg.Id("result").Op(":=").Make(Index().Add(elementTypeCode), Lit(0), Len(Id("parts")))
			bg.For(List(Id("_"), Id("elemStr")).Op(":=").Range().Id("parts")).BlockFunc(func(ig *Group) {
				ig.Id("elemStr").Op("=").Qual(PackageStrings, "TrimSpace").Call(Id("elemStr"))
				ig.If(Id("elemStr").Op("==").Lit("")).Block(
					Continue(),
				)
				ig.Var().Id("elem").Add(elementTypeCode)
				ig.Add(r.argToTypeConverter(srcFile, typeGen, Id("elemStr"), elementVar, Id("elem"), errStatement))
				ig.Id("result").Op("=").Append(Id("result"), Id("elem"))
			})
			bg.Add(id).Op("=").Id("result")
		})
	}

	// Для встроенных типов TypeID - это имя типа, они не в project.Types
	if isBuiltinTypeID(arg.TypeID) {
		switch arg.TypeID {
		case "string":
			return id.Op(op).Add(from)
		case "bool":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseBool").Call(from).Add(errStatement)
		case "int":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "Atoi").Call(from).Add(errStatement)
		case "int64":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseInt").Call(from, Lit(10), Lit(64)).Add(errStatement)
		case "int32":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseInt").Call(from, Lit(10), Lit(32)).Add(errStatement)
		case "uint":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseUint").Call(from, Lit(10), Lit(64)).Add(errStatement)
		case "uint64":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseUint").Call(from, Lit(10), Lit(64)).Add(errStatement)
		case "uint32":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseUint").Call(from, Lit(10), Lit(32)).Add(errStatement)
		case "float64":
			return List(id, Err()).Op(op).Qual(PackageStrconv, "ParseFloat").Call(from, Lit(64)).Add(errStatement)
		case "float32":
			temp64 := Id("temp64")
			return List(temp64, Err()).Op(":=").Qual(PackageStrconv, "ParseFloat").Call(from, Lit(32)).Add(errStatement).Add(Line()).Add(id.Op(op).Float32().Call(temp64))
		default:
			// Для остальных встроенных типов (byte, rune, error, any) используем прямое преобразование
			return id.Op(op).Add(from)
		}
	}

	// Пользовательский тип: ищем в project.Types
	typ, ok := r.project.Types[arg.TypeID]
	if !ok {
		// Если тип не найден, используем прямое преобразование
		return id.Op(op).Add(from)
	}

	// Используем централизованную логику из core для определения формата сериализации
	openAPIType, format := core.GetSerializationFormat(typ, r.project)

	// Определяем, как парсить тип на основе формата сериализации
	switch {
	case openAPIType == "string" && format == "uuid":
		// UUID - парсим через uuid.Parse
		uuidPackage := PackageUUID
		return List(id, Id("_")).Op(op).Qual(uuidPackage, "Parse").Call(from)
	case openAPIType == "string" && format == "date-time":
		// time.Time - парсим через time.Parse
		return List(id, Err()).Op(op).Qual(PackageTime, "Parse").Call(Qual(PackageTime, "RFC3339Nano"), from).Add(errStatement)
	case openAPIType == "string" && containsString(typ.ImplementsInterfaces, "encoding/json.Marshaler"):
		// Типы, реализующие json.Marshaler, парсим через JSON unmarshal
		jsonPkg := PackageStdJSON
		return Op("_").Op("=").Qual(jsonPkg, "Unmarshal").Call(Op("[]").Byte().Call(Op("`\"`").Op("+").Add(from).Op("+").Op("`\"`")), Op("&").Add(id))
	default:
		// Для остальных сложных типов используем JSON unmarshal
		jsonPkg := PackageStdJSON
		return Op("_").Op("=").Qual(jsonPkg, "Unmarshal").Call(Op("[]").Byte().Call(Op("`\"`").Op("+").Add(from).Op("+").Op("`\"`")), Op("&").Add(id))
	}
}

// containsString проверяет, содержится ли строка в слайсе.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// toIDWithImport генерирует код для вызова функции с импортом пакета.
func toIDWithImport(qualifiedName string, srcFile *GoFile) *Statement {
	// Формат: "package/path:FunctionName"
	if tokens := strings.Split(qualifiedName, ":"); len(tokens) == 2 {
		pkgPath := tokens[0]
		funcName := tokens[1]
		baseName := filepath.Base(pkgPath)
		srcFile.ImportName(pkgPath, baseName)
		return Qual(pkgPath, funcName)
	}
	// Если формат неверный, возвращаем как есть
	return Id(qualifiedName)
}

// arguments возвращает аргументы без context и специальных параметров.
func (r *contractRenderer) arguments(method *core.Method) []*core.Variable {
	return r.argsWithoutSpecialArgs(method)
}

// urlArgs генерирует код для извлечения аргументов из path параметров.
func (r *contractRenderer) urlArgs(srcFile *GoFile, typeGen *types.Generator, method *core.Method, errStatement func(arg, header string) *Statement) *Statement {
	return r.argFromString(srcFile, typeGen, method, "urlParam", r.argPathMap(method),
		func(srcName string) Code {
			return Id(VarNameFtx).Dot("Params").Call(Lit(srcName))
		},
		errStatement,
	)
}

// urlParams генерирует код для извлечения аргументов из query параметров.
func (r *contractRenderer) urlParams(srcFile *GoFile, typeGen *types.Generator, method *core.Method, errStatement func(arg, header string) *Statement) *Statement {
	queryParams := make(map[string]string)
	if urlArgs := method.Annotations.Value(TagHttpArg, ""); urlArgs != "" {
		paramPairs := strings.Split(urlArgs, ",")
		for _, pair := range paramPairs {
			pair = strings.TrimSpace(pair)
			if strings.HasPrefix(pair, "query|") {
				if pairTokens := strings.Split(pair, "|"); len(pairTokens) >= 3 {
					arg := strings.TrimSpace(pairTokens[1])
					param := strings.TrimSpace(pairTokens[2])
					queryParams[arg] = param
				}
			} else if strings.Contains(pair, "|") && !strings.HasPrefix(strings.TrimSpace(pair), "path|") && !strings.HasPrefix(strings.TrimSpace(pair), "header|") && !strings.HasPrefix(strings.TrimSpace(pair), "cookie|") {
				if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
					arg := strings.TrimSpace(pairTokens[0])
					param := strings.TrimSpace(pairTokens[1])
					if _, inPath := r.argPathMap(method)[arg]; !inPath {
						if _, inHeader := r.varHeaderMap(method)[arg]; !inHeader {
							if _, inCookie := r.varCookieMap(method)[arg]; !inCookie {
								queryParams[arg] = param
							}
						}
					}
				}
			}
		}
	}
	var orderedArgs []string
	if urlArgs := method.Annotations.Value(TagHttpArg, ""); urlArgs != "" {
		paramPairs := strings.Split(urlArgs, ",")
		for _, pair := range paramPairs {
			pair = strings.TrimSpace(pair)
			if strings.HasPrefix(pair, "query|") {
				if pairTokens := strings.Split(pair, "|"); len(pairTokens) >= 3 {
					arg := strings.TrimSpace(pairTokens[1])
					if _, ok := queryParams[arg]; ok {
						orderedArgs = append(orderedArgs, arg)
					}
				}
			} else if strings.Contains(pair, "|") && !strings.HasPrefix(pair, "path|") && !strings.HasPrefix(pair, "header|") && !strings.HasPrefix(pair, "cookie|") {
				if pairTokens := strings.Split(pair, "|"); len(pairTokens) == 2 {
					arg := strings.TrimSpace(pairTokens[0])
					if _, ok := queryParams[arg]; ok {
						orderedArgs = append(orderedArgs, arg)
					}
				}
			}
		}
	}
	return r.argFromStringOrdered(srcFile, typeGen, method, "queryParam", queryParams, orderedArgs,
		func(srcName string) Code {
			return Id(VarNameFtx).Dot("Query").Call(Lit(srcName))
		},
		errStatement,
	)
}

// httpArgHeaders генерирует код для извлечения аргументов из HTTP заголовков.
func (r *contractRenderer) httpArgHeaders(srcFile *GoFile, typeGen *types.Generator, method *core.Method, errStatement func(arg, header string) *Statement) *Statement {
	return r.argFromString(srcFile, typeGen, method, "header", r.varHeaderMap(method),
		func(srcName string) Code {
			return Id(VarNameFtx).Dot("Get").Call(Lit(srcName))
		},
		errStatement,
	)
}

// httpCookies генерирует код для извлечения аргументов из HTTP cookies.
func (r *contractRenderer) httpCookies(srcFile *GoFile, typeGen *types.Generator, method *core.Method, errStatement func(arg, header string) *Statement) *Statement {
	return r.argFromString(srcFile, typeGen, method, "cookie", r.varCookieMap(method),
		func(srcName string) Code {
			return Id(VarNameFtx).Dot("Cookies").Call(Lit(srcName))
		},
		errStatement,
	)
}

// httpRetHeaders генерирует код для установки HTTP заголовков из результатов.
func (r *contractRenderer) httpRetHeaders(method *core.Method) *Statement {
	ex := Line()
	headerMap := r.varHeaderMap(method)
	for varName, headerName := range headerMap {
		if ret := r.resultByName(method, varName); ret != nil {
			ex.Id(VarNameFtx).Dot("Set").Call(Lit(headerName), Id("response").Dot(toCamel(varName)))
		}
	}
	return ex
}

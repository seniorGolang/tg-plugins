// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/core"
)

// RenderClientTypes генерирует локальные версии всех типов, используемых в exchange структурах.
// Использует project.Types напрямую, проверяя для каждого typeID, нужно ли генерировать локально.
func (r *ClientRenderer) RenderClientTypes(collectedTypeIDs map[string]bool) error {

	if len(collectedTypeIDs) == 0 {
		// Нет типов для генерации
		return nil
	}

	// Создаем директорию dto, если её нет
	dtoDir := path.Join(r.outDir, "dto")
	if err := os.MkdirAll(dtoDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию dto: %w", err)
	}

	srcFile := NewSrcFile("dto")
	srcFile.PackageComment(DoNotEdit)

	ctx := context.WithValue(context.Background(), keyCode, srcFile) // nolint
	ctx = context.WithValue(ctx, keyPackage, "dto")                  // nolint

	// Генерируем типы в детерминированном порядке (сортируем по typeID)
	typeIDs := make([]string, 0, len(collectedTypeIDs))
	for typeID := range collectedTypeIDs {
		typeIDs = append(typeIDs, typeID)
	}

	// Сортируем для детерминированного порядка
	for i := 0; i < len(typeIDs)-1; i++ {
		for j := i + 1; j < len(typeIDs); j++ {
			if typeIDs[i] > typeIDs[j] {
				typeIDs[i], typeIDs[j] = typeIDs[j], typeIDs[i]
			}
		}
	}

	// Генерируем каждый тип в отдельный файл
	// Используем project.Types напрямую, проверяя для каждого typeID, нужно ли генерировать локально
	for _, typeID := range typeIDs {
		// Получаем тип из project.Types (Core уже собрал все типы рекурсивно)
		typ, ok := r.project.Types[typeID]
		if !ok {
			continue
		}

		// Пропускаем встроенные типы
		if r.isBuiltinType(typeID) {
			continue
		}

		// Определяем, является ли тип из текущего проекта
		isFromCurrentProject := r.isTypeFromCurrentProject(typ.ImportPkgPath)

		// Генерируем типы из текущего проекта
		// ВАЖНО: алиасы на внешние типы тоже генерируем, но как алиасы (type Alias = ExternalType)
		// Это позволяет сохранить семантику алиаса в клиенте
		if !isFromCurrentProject {
			// Для внешних типов генерируем только алиасы
			if typ.Kind == core.TypeKindAlias && typ.AliasOf != "" {
				// Алиас на внешний тип - генерируем как алиас
			} else {
				// Остальные внешние типы не генерируем
				continue
			}
		}

		// ВАЖНО: пропускаем анонимные типы (interface:anonymous, struct:anonymous и т.д.)
		// так как они не могут быть сгенерированы как отдельные типы
		if strings.Contains(typeID, ":interface:anonymous") ||
			strings.Contains(typeID, ":struct:anonymous") ||
			strings.Contains(typeID, ":func:anonymous") {
			continue
		}

		// Получаем имя типа для имени файла
		typeName := typ.TypeName
		if typeName == "" {
			// Извлекаем имя из typeID (формат "pkgPath:TypeName" или просто "TypeName")
			if strings.Contains(typeID, ":") {
				parts := strings.SplitN(typeID, ":", 2)
				if len(parts) == 2 {
					typeName = parts[1]
					// ВАЖНО: для анонимных типов (interface:anonymous, struct:anonymous и т.д.)
					// не генерируем отдельный файл, так как это невалидное имя типа
					if strings.Contains(typeName, ":") || typeName == "anonymous" || typeName == "interface" || typeName == "struct" || typeName == "func" {
						continue
					}
				} else {
					typeName = typeID
				}
			} else {
				typeName = typeID
			}
		}

		// ВАЖНО: если тип собран, он ДОЛЖЕН быть сгенерирован
		// Генерируем тип напрямую в зависимости от его структуры
		var typeCode Code
		switch {
		case typ.Kind == core.TypeKindStruct:
			typeCode = r.generateClientStruct(ctx, typeName, typ)
		case typ.Kind == core.TypeKindInterface:
			typeCode = r.generateClientInterface(ctx, typeName, typ)
		case typ.Kind == core.TypeKindAlias:
			// Алиасы (type ID = string)
			// ВАЖНО: алиасы всегда генерируем, чтобы сохранить семантику (type Alias = BaseType)
			typeCode = r.generateClientAlias(ctx, typeName, typ)
		case typ.ImportPkgPath != "" && typ.TypeName != "":
			// Именованные типы с базовым типом (type UserID int64, type Email string)
			// Имеют Kind как базовый тип, но ImportPkgPath и TypeName указывают на именованный тип
			typeCode = r.generateClientAlias(ctx, typeName, typ)
		default:
			// Для остальных типов (массивы, мапы, встроенные базовые типы без имени) не генерируем
			continue
		}

		if typeCode != nil {
			// Создаем отдельный файл для каждого типа
			typeFile := NewSrcFile("dto")
			typeFile.PackageComment(DoNotEdit)
			typeCtx := context.WithValue(context.Background(), keyCode, typeFile) // nolint
			typeCtx = context.WithValue(typeCtx, keyPackage, "dto")               // nolint

			// ВАЖНО: перегенерируем typeCode с правильным контекстом для алиасов
			// Это нужно для правильной установки ImportName в правильном файле
			// Согласно JENNIFER_IMPORTS_GUIDE.md: ImportName должен вызываться с правильным srcFile в контексте
			if typ.Kind == core.TypeKindAlias && typ.AliasOf != "" {
				typeCode = r.generateClientAlias(typeCtx, typeName, typ)
			}

			typeFile.Add(typeCode)

			// Сохраняем файл с именем типа (в нижнем регистре)
			fileName := strings.ToLower(typeName) + ".go"
			if err := typeFile.Save(path.Join(dtoDir, fileName)); err != nil {
				return fmt.Errorf("не удалось сохранить файл типа %s: %w", typeName, err)
			}
		}
	}

	return nil
}

// generateClientStruct генерирует код для структуры.
func (r *ClientRenderer) generateClientStruct(ctx context.Context, typeName string, typ *core.Type) Code {

	return Type().Id(typeName).StructFunc(func(gr *Group) {
		for _, field := range typ.StructFields {
			fieldCode := r.generateClientStructField(ctx, field)
			gr.Add(fieldCode)
		}
	})
}

// generateClientStructField генерирует код для поля структуры.
func (r *ClientRenderer) generateClientStructField(ctx context.Context, field *core.StructField) *Statement {

	// Обрабатываем встроенные поля (embedded fields) - когда Name пустой
	var s *Statement
	if field.Name == "" {
		// Встроенное поле - генерируем только тип без имени
		s = &Statement{}
	} else {
		// Обычное поле - генерируем с именем
		fieldName := ToCamel(field.Name)
		s = Id(fieldName)
	}

	// Обрабатываем в зависимости от вида поля
	switch {
	case field.IsSlice || field.ArrayLen > 0:
		// Обрабатываем массивы и слайсы
		if field.IsSlice {
			s = s.Index()
		} else {
			s = s.Index(Lit(field.ArrayLen))
		}
		if field.TypeID != "" {
			// Для массивов указатели применяются к типу элемента
			// ВАЖНО: используем ElementPointers, а не NumberOfPointers
			s = s.Add(r.fieldTypeForClient(ctx, field.TypeID, field.ElementPointers, false))
		}

	case field.MapKeyID != "" && field.MapValueID != "":
		// Обрабатываем мапы
		keyType := r.fieldTypeForClient(ctx, field.MapKeyID, field.MapKeyPointers, false)
		valueType := r.fieldTypeForClient(ctx, field.MapValueID, field.ElementPointers, false)
		s = s.Map(keyType).Add(valueType)

	default:
		// Обычное поле - указатели применяются к типу
		// ВАЖНО: проверяем анонимные интерфейсы и пустые typeID перед вызовом fieldTypeForClient
		switch {
		case field.TypeID == "":
			// Пустой typeID обычно означает interface{} или any
			s = s.Id("any")
		case strings.Contains(field.TypeID, ":interface:anonymous"):
			s = s.Id("any")
		default:
			s = s.Add(r.fieldTypeForClient(ctx, field.TypeID, field.NumberOfPointers, false))
		}
	}

	// Добавляем теги
	tags := make(map[string]string)
	for tagName, tagValues := range field.Tags {
		if len(tagValues) > 0 {
			tags[tagName] = strings.Join(tagValues, ",")
		}
	}
	if len(tags) > 0 {
		s = s.Tag(tags)
	}

	// Добавляем комментарии
	if len(field.Docs) > 0 {
		for _, doc := range field.Docs {
			s = s.Comment(doc)
		}
	}

	return s
}

// generateClientInterface генерирует код для интерфейса.
func (r *ClientRenderer) generateClientInterface(ctx context.Context, typeName string, typ *core.Type) Code {

	return Type().Id(typeName).InterfaceFunc(func(gr *Group) {
		// Добавляем встроенные интерфейсы
		for _, embedded := range typ.EmbeddedInterfaces {
			embeddedType := r.fieldTypeForClient(ctx, embedded.TypeID, 0, false)
			gr.Add(embeddedType)
		}

		// Добавляем методы
		for _, method := range typ.InterfaceMethods {
			methodCode := r.generateClientMethod(ctx, method)
			gr.Add(methodCode)
		}
	})
}

// generateClientMethod генерирует код для метода интерфейса.
func (r *ClientRenderer) generateClientMethod(ctx context.Context, method *core.Function) *Statement {

	s := Id(method.Name)

	// Параметры
	args := make([]Code, 0, len(method.Args))
	for _, arg := range method.Args {
		argType := r.fieldTypeFromVariableForClient(ctx, arg, false)
		argName := arg.Name
		if argName == "" {
			argName = "_"
		} else {
			argName = ToLowerCamel(argName)
		}
		args = append(args, Id(argName).Add(argType))
	}

	// Результаты
	results := make([]Code, 0, len(method.Results))
	for _, result := range method.Results {
		resultType := r.fieldTypeFromVariableForClient(ctx, result, false)
		results = append(results, resultType)
	}

	s = s.Params(args...)
	if len(results) > 0 {
		s = s.Params(results...)
	}

	return s
}

// generateClientAlias генерирует код для алиаса типа или именованного типа с базовым типом.
func (r *ClientRenderer) generateClientAlias(ctx context.Context, typeName string, typ *core.Type) Code {

	// Для алиасов всегда используем базовый тип через AliasOf
	// Это позволяет сохранить семантику алиаса в клиенте (type Alias = BaseType)
	if typ.AliasOf != "" {
		// Используем базовый тип - он будет правильно обработан (импорт или локальный тип)
		baseType := r.fieldTypeForClient(ctx, typ.AliasOf, 0, false)
		return Type().Id(typeName).Op("=").Add(baseType)
	}
	if typ.UnderlyingTypeID != "" {
		baseType := r.fieldTypeForClient(ctx, typ.UnderlyingTypeID, 0, false)
		return Type().Id(typeName).Op("=").Add(baseType)
	}

	// Обрабатываем map типы отдельно
	if typ.Kind == core.TypeKindMap {
		if typ.MapKeyID != "" && typ.MapValueID != "" {
			keyType := r.fieldTypeForClient(ctx, typ.MapKeyID, typ.MapKeyPointers, false)
			valueType := r.fieldTypeForClient(ctx, typ.MapValueID, typ.ElementPointers, false)
			return Type().Id(typeName).Op("=").Map(keyType).Add(valueType)
		}
		// Если MapKeyID или MapValueID пустые, используем string для ключа и any для значения
		return Type().Id(typeName).Op("=").Map(Id("string")).Id("any")
	}

	// Для именованных типов с базовым типом (type UserID int64) используем UnderlyingKind
	if typ.UnderlyingKind != "" {
		return Type().Id(typeName).Id(string(typ.UnderlyingKind))
	}

	// Fallback на базовый Kind (если UnderlyingKind не установлен, используем Kind)
	// Но не для map типов - они уже обработаны выше
	if typ.Kind != core.TypeKindMap {
		return Type().Id(typeName).Id(string(typ.Kind))
	}

	// Если дошли сюда и это map, используем string для ключа и any для значения
	return Type().Id(typeName).Op("=").Map(Id("string")).Id("any")
}

// fieldTypeForClient генерирует тип для клиента, используя локальные версии вместо импорта.
// Это версия fieldType, которая использует локальные типы из dto пакета для типов из текущего проекта.
func (r *ClientRenderer) fieldTypeForClient(ctx context.Context, typeID string, numberOfPointers int, allowEllipsis bool) *Statement {

	c := &Statement{}

	// Добавляем указатели
	for i := 0; i < numberOfPointers; i++ {
		c.Op("*")
	}

	// Получаем тип из проекта
	typ, ok := r.project.Types[typeID]
	if !ok {
		// Тип не найден - проверяем, является ли он встроенным
		if r.isBuiltinType(typeID) {
			return c.Id(typeID)
		}
		// ВАЖНО: проверяем анонимные типы (interface:anonymous, struct:anonymous и т.д.)
		if strings.Contains(typeID, ":interface:anonymous") {
			return c.Id("any")
		}
		// Если тип не найден, но это внешний тип (содержит ":"), используем его как именованный тип
		if strings.Contains(typeID, ":") {
			parts := strings.SplitN(typeID, ":", 2)
			if len(parts) == 2 {
				importPkgPath := parts[0]
				typeName := parts[1]
				// Если это внешний тип (не из текущего проекта), используем его как именованный тип
				if !r.isTypeFromCurrentProject(importPkgPath) {
					if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
						packageName := filepath.Base(importPkgPath)
						srcFile.ImportName(importPkgPath, packageName)
						return c.Qual(importPkgPath, typeName)
					}
					return c.Qual(importPkgPath, typeName)
				}
			}
		}
		// Если тип не найден в project.Types, это ошибка - Core должен был обработать все типы
		// Возвращаем typeID как fallback
		return c.Id(typeID)
	}

	// ВАЖНО: для типов из внешних пакетов (не из текущего проекта) используем их как именованные типы,
	// независимо от Kind. Например, uuid.UUID имеет Kind == TypeKindArray, но это именованный тип
	// из внешнего пакета, и его нужно использовать как uuid.UUID, а не как [16]byte
	// Эта проверка должна быть ПЕРВОЙ, до всех остальных обработок (ellipsis, switch по Kind и т.д.)
	if typ.ImportPkgPath != "" && typ.TypeName != "" {
		if !r.isTypeFromCurrentProject(typ.ImportPkgPath) {
			// Тип из внешнего пакета - используем информацию из shared напрямую
			// Согласно JENNIFER_IMPORTS_GUIDE.md: используем ImportName с PkgName из shared
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				// PkgName содержит реальное имя пакета из package декларации (например, "uuid")
				packageName := typ.PkgName
				if packageName == "" {
					// Fallback на последнюю часть пути, если PkgName не установлен
					packageName = filepath.Base(typ.ImportPkgPath)
				}
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
			return c.Add(r.fieldTypeForClient(ctx, typ.ArrayOfID, 0, false))
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
			return c.Add(r.fieldTypeForClient(ctx, typ.ArrayOfID, 0, false))
		}
		return c

	case core.TypeKindMap:
		// ВАЖНО: если это именованный map тип из текущего проекта, используем имя типа
		if typ.TypeName != "" && typ.ImportPkgPath != "" {
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта - используем имя типа
				if currentPkg, ok := ctx.Value(keyPackage).(string); ok && currentPkg == "dto" {
					return c.Id(typ.TypeName)
				}
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
		}
		// Для неименованных map типов генерируем map напрямую
		if typ.MapKeyID != "" && typ.MapValueID != "" {
			keyType := r.fieldTypeForClient(ctx, typ.MapKeyID, typ.MapKeyPointers, false)
			valueType := r.fieldTypeForClient(ctx, typ.MapValueID, typ.ElementPointers, false)
			return c.Map(keyType).Add(valueType)
		}
		// Если MapKeyID или MapValueID пустые, используем string для ключа и any для значения
		return c.Map(Id("string")).Id("any")

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
			return chanType.Add(r.fieldTypeForClient(ctx, typ.ChanOfID, 0, false))
		}
		return chanType

	case core.TypeKindStruct, core.TypeKindInterface:
		// ВАЖНО: все типы из текущего проекта должны генерироваться локально и использоваться из dto пакета
		if typ.ImportPkgPath != "" && typ.TypeName != "" {
			// Проверяем, является ли тип из текущего проекта
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта
				// ВАЖНО: если мы генерируем код в пакете dto, то типы из того же пакета
				// используем без импорта (просто по имени), чтобы избежать циклических импортов
				if currentPkg, ok := ctx.Value(keyPackage).(string); ok && currentPkg == "dto" {
					// Генерируем код в пакете dto - используем просто имя типа без импорта
					return c.Id(typ.TypeName)
				}
				// Тип из текущего проекта, но не генерируется в пакете dto - используем dto пакет
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
			// Тип из внешнего пакета - используем информацию из shared напрямую
			// Согласно JENNIFER_IMPORTS_GUIDE.md: используем ImportName с PkgName из shared
			// ImportName устанавливает имя пакета для использования в Qual
			// Если имя пакета совпадает с используемым в коде, jennifer НЕ добавляет псевдоним
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				// PkgName содержит реальное имя пакета из package декларации (например, "jose")
				packageName := typ.PkgName
				if packageName == "" {
					// Fallback на последнюю часть пути, если PkgName не установлен
					packageName = filepath.Base(typ.ImportPkgPath)
				}
				srcFile.ImportName(typ.ImportPkgPath, packageName)
				return c.Qual(typ.ImportPkgPath, typ.TypeName)
			}
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
		// Если TypeName пустой, это может быть анонимный интерфейс (interface{})
		if strings.Contains(typeID, ":interface:anonymous") || typ.Kind == core.TypeKindAny {
			return c.Id("any")
		}
		// Если нет ImportPkgPath, используем TypeName напрямую
		if typ.TypeName != "" {
			return c.Id(typ.TypeName)
		}
		// Fallback на any для пустых интерфейсов
		return c.Id("any")

	case core.TypeKindFunction:
		args := make([]Code, 0, len(typ.FunctionArgs))
		for _, arg := range typ.FunctionArgs {
			argType := r.fieldTypeFromVariableForClient(ctx, arg, false)
			args = append(args, argType)
		}
		results := make([]Code, 0, len(typ.FunctionResults))
		for _, res := range typ.FunctionResults {
			resType := r.fieldTypeFromVariableForClient(ctx, res, false)
			results = append(results, resType)
		}
		return c.Func().Params(args...).Params(results...)

	case core.TypeKindAlias:
		// ВАЖНО: для алиасов из текущего проекта используем локальный тип из dto пакета
		// Это позволяет сохранить семантику алиаса в клиенте
		if typ.ImportPkgPath != "" && typ.TypeName != "" {
			// Проверяем, является ли тип из текущего проекта
			if r.isTypeFromCurrentProject(typ.ImportPkgPath) {
				// Тип из текущего проекта - используем локальный тип из dto пакета
				if currentPkg, ok := ctx.Value(keyPackage).(string); ok && currentPkg == "dto" {
					// Генерируем код в пакете dto - используем просто имя типа без импорта
					return c.Id(typ.TypeName)
				}
				// Тип из текущего проекта, но не генерируется в пакете dto - используем dto пакет
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
		}
		// Для внешних алиасов или если нет ImportPkgPath - используем базовый тип
		// НО: если это алиас из текущего проекта, но ImportPkgPath не установлен,
		// проверяем, есть ли TypeName и используем его
		if typ.AliasOf != "" {
			// ВАЖНО: если алиас имеет TypeName и ImportPkgPath из текущего проекта,
			// используем имя алиаса, а не базовый тип, независимо от того, откуда базовый тип
			if typ.TypeName != "" {
				// Проверяем, является ли алиас из текущего проекта
				isAliasFromCurrentProject := false
				if typ.ImportPkgPath != "" {
					isAliasFromCurrentProject = r.isTypeFromCurrentProject(typ.ImportPkgPath)
				} else {
					// Если ImportPkgPath не установлен, проверяем по базовому типу
					if baseType, exists := r.project.Types[typ.AliasOf]; exists {
						isAliasFromCurrentProject = r.isTypeFromCurrentProject(baseType.ImportPkgPath)
					}
				}

				if isAliasFromCurrentProject {
					// Алиас из текущего проекта - используем имя алиаса
					if currentPkg, ok := ctx.Value(keyPackage).(string); ok && currentPkg == "dto" {
						return c.Id(typ.TypeName)
					}
					if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
						dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
						srcFile.ImportName(dtoPkgPath, "dto")
						return c.Qual(dtoPkgPath, typ.TypeName)
					}
					return c.Id(typ.TypeName)
				}
			}
			// Для внешних алиасов используем базовый тип
			if baseType, exists := r.project.Types[typ.AliasOf]; exists {
				importPkgPath := baseType.ImportPkgPath
				typeName := baseType.TypeName
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					// PkgName содержит реальное имя пакета из package декларации
					packageName := baseType.PkgName
					if packageName == "" {
						packageName = filepath.Base(importPkgPath)
					}
					srcFile.ImportName(importPkgPath, packageName)
				}
				return c.Qual(importPkgPath, typeName)
			}
			return r.fieldTypeForClient(ctx, typ.AliasOf, numberOfPointers, allowEllipsis)
		}
		// Если TypeName есть, но нет AliasOf, используем TypeName напрямую
		if typ.TypeName != "" {
			if currentPkg, ok := ctx.Value(keyPackage).(string); ok && currentPkg == "dto" {
				return c.Id(typ.TypeName)
			}
		}
		if typ.UnderlyingTypeID != "" {
			return r.fieldTypeForClient(ctx, typ.UnderlyingTypeID, numberOfPointers, allowEllipsis)
		}
		// Fallback на UnderlyingKind
		if typ.UnderlyingKind != "" {
			return c.Id(string(typ.UnderlyingKind))
		}
		// Если ничего не найдено, используем Kind
		return c.Id(string(typ.Kind))

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
				// Тип из текущего проекта
				// ВАЖНО: если мы генерируем код в пакете dto, то типы из того же пакета
				// используем без импорта (просто по имени), чтобы избежать циклических импортов
				if currentPkg, ok := ctx.Value(keyPackage).(string); ok && currentPkg == "dto" {
					// Генерируем код в пакете dto - используем просто имя типа без импорта
					return c.Id(typ.TypeName)
				}
				// Тип из текущего проекта, но не генерируется в пакете dto - используем dto пакет
				if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
					dtoPkgPath := fmt.Sprintf("%s/dto", r.pkgPath(r.outDir))
					srcFile.ImportName(dtoPkgPath, "dto")
					return c.Qual(dtoPkgPath, typ.TypeName)
				}
				return c.Id(typ.TypeName)
			}
			// Тип из внешнего пакета (например, time.Time) - используем информацию из shared напрямую
			// Согласно JENNIFER_IMPORTS_GUIDE.md: используем ImportName с PkgName из shared
			if srcFile, ok := ctx.Value(keyCode).(GoFile); ok {
				// PkgName содержит реальное имя пакета из package декларации
				packageName := typ.PkgName
				if packageName == "" {
					packageName = filepath.Base(typ.ImportPkgPath)
				}
				srcFile.ImportName(typ.ImportPkgPath, packageName)
			}
			return c.Qual(typ.ImportPkgPath, typ.TypeName)
		}
		// Встроенный базовый тип - используем Kind как имя типа
		return c.Id(string(typ.Kind))

	default:
		return c
	}
}

// isBuiltinType проверяет, является ли тип встроенным типом Go.
func (r *ClientRenderer) isBuiltinType(typeID string) bool {
	builtinTypes := map[string]bool{
		"string":  true,
		"int":     true,
		"int8":    true,
		"int16":   true,
		"int32":   true,
		"int64":   true,
		"uint":    true,
		"uint8":   true,
		"uint16":  true,
		"uint32":  true,
		"uint64":  true,
		"float32": true,
		"float64": true,
		"bool":    true,
		"byte":    true,
		"rune":    true,
		"error":   true,
		"any":     true,
	}
	return builtinTypes[typeID]
}

// fieldTypeFromVariableForClient генерирует тип из Variable для клиента.
func (r *ClientRenderer) fieldTypeFromVariableForClient(ctx context.Context, variable *core.Variable, allowEllipsis bool) *Statement {

	c := &Statement{}

	// Обрабатываем ellipsis
	if variable.IsEllipsis && allowEllipsis {
		// Добавляем указатели перед ellipsis
		for i := 0; i < variable.NumberOfPointers; i++ {
			c.Op("*")
		}
		c.Op("...")
		if variable.TypeID != "" {
			return c.Add(r.fieldTypeForClient(ctx, variable.TypeID, 0, false))
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
						srcFile.ImportName(typ.ImportPkgPath, packageName)
						return c.Qual(typ.ImportPkgPath, typ.TypeName)
					}
					return c.Qual(typ.ImportPkgPath, typ.TypeName)
				}
			}
		}
		// Добавляем указатели на переменной (если есть *[]T)
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
			return c.Add(r.fieldTypeForClient(ctx, variable.TypeID, variable.ElementPointers, false))
		}
		return c.Add(Id("any"))
	}

	// Обрабатываем map
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		// Указатели на переменной применяются к map
		for i := 0; i < variable.NumberOfPointers; i++ {
			c.Op("*")
		}
		keyType := r.fieldTypeForClient(ctx, variable.MapKeyID, variable.MapKeyPointers, false)
		valueType := r.fieldTypeForClient(ctx, variable.MapValueID, variable.ElementPointers, false)
		return c.Map(keyType).Add(valueType)
	}

	// Базовый тип
	return c.Add(r.fieldTypeForClient(ctx, variable.TypeID, 0, false))
}

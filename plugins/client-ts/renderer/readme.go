// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"

	"tgp/internal/markdown"

	"tgp/shared"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// DocOptions содержит опции для генерации документации
type DocOptions struct {
	Enabled  bool   // Включена ли генерация документации (по умолчанию true)
	FilePath string // Полный путь к файлу документации (пусто = outDir/README.md)
}

// RenderReadmeTS генерирует README.md для TypeScript клиента
func (r *ClientRenderer) RenderReadmeTS(docOpts DocOptions) error {
	var err error
	outDir := r.outDir

	if !docOpts.Enabled {
		return nil
	}

	var buf bytes.Buffer
	md := markdown.NewMarkdown(&buf)

	// Заголовок
	md.H1("API Документация")
	md.PlainText("Автоматически сгенерированная документация API для TypeScript клиента.")

	// Сортируем контракты по имени для консистентности
	contracts := make([]*shared.Contract, len(r.project.Contracts))
	copy(contracts, r.project.Contracts)
	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].Name < contracts[j].Name
	})

	hasJsonRPC := false

	// Генерируем оглавление
	md.H2("Оглавление")
	md.PlainText(fmt.Sprintf("- [Описание клиента](#%s)", generateAnchor("Описание клиента")))
	md.LF()
	md.PlainText(markdown.Bold("Контракты:"))
	md.LF()

	// Генерируем оглавление контрактов и их методов
	for _, contract := range contracts {
		if r.contains(contract.Annotations, TagServerJsonRPC) {
			hasJsonRPC = true
		}

		contractAnchor := generateAnchor(contract.Name)
		md.PlainText(fmt.Sprintf("- [%s](#%s)", contract.Name, contractAnchor))
		md.LF()

		// JSON-RPC методы
		if r.contains(contract.Annotations, TagServerJsonRPC) {
			for _, method := range contract.Methods {
				if r.methodIsJsonRPC(contract, method) {
					methodAnchor := generateAnchor(method.Name)
					md.PlainText(fmt.Sprintf("  - [%s](#%s)", method.Name, methodAnchor))
					md.LF()
				}
			}
		}

		// HTTP методы
		if r.contains(contract.Annotations, TagServerHTTP) {
			for _, method := range contract.Methods {
				if r.methodIsHTTP(method) {
					httpMethod := "GET"
					if val, ok := method.Annotations[TagMethodHTTP]; ok {
						httpMethod = val
					}
					httpPath := ""
					if val, ok := method.Annotations[TagHttpPath]; ok {
						httpPath = val
					}
					methodTitle := fmt.Sprintf("%s %s", httpMethod, httpPath)
					methodAnchor := generateAnchor(methodTitle)
					md.PlainText(fmt.Sprintf("  - [%s](#%s)", methodTitle, methodAnchor))
					md.LF()
				}
			}
		}
	}
	md.LF()

	// Собираем все используемые типы
	typeUsages := r.collectStructTypesTS()
	allTypes := make(map[string]*typeUsageTS)
	for key, usage := range typeUsages {
		allTypes[key] = usage
	}

	// Сортируем типы для оглавления
	typeKeys := make([]string, 0, len(allTypes))
	for key := range allTypes {
		typeKeys = append(typeKeys, key)
	}
	sort.Strings(typeKeys)

	// Типы данных
	if len(allTypes) > 0 {
		md.PlainText(markdown.Bold("Типы данных:"))
		md.LF()
		md.PlainText(fmt.Sprintf("- [Общие типы](#%s)", generateAnchor("Общие типы")))
		md.LF()
		
		// Группируем типы по namespace для оглавления
		typesByNamespaceTOC := make(map[string][]*typeUsageTS)
		for _, usage := range allTypes {
			namespace := ""
			if strings.Contains(usage.fullTypeName, ".") {
				parts := strings.SplitN(usage.fullTypeName, ".", 2)
				namespace = parts[0]
			}
			if typesByNamespaceTOC[namespace] == nil {
				typesByNamespaceTOC[namespace] = make([]*typeUsageTS, 0)
			}
			typesByNamespaceTOC[namespace] = append(typesByNamespaceTOC[namespace], usage)
		}
		
		// Сортируем namespace для оглавления
		namespaceKeysTOC := make([]string, 0, len(typesByNamespaceTOC))
		for ns := range typesByNamespaceTOC {
			namespaceKeysTOC = append(namespaceKeysTOC, ns)
		}
		sort.Strings(namespaceKeysTOC)
		
		// Добавляем ссылки на типы, сгруппированные по namespace
		for _, namespace := range namespaceKeysTOC {
			types := typesByNamespaceTOC[namespace]
			sort.Slice(types, func(i, j int) bool {
				return types[i].fullTypeName < types[j].fullTypeName
			})
			
			// Если есть namespace, добавляем подзаголовок
			if namespace != "" {
				namespaceAnchor := generateAnchor(namespace)
				md.PlainText(fmt.Sprintf("  - [%s](#%s)", namespace, namespaceAnchor))
				md.LF()
			}
			
			// Добавляем ссылки на конкретные типы
			for _, usage := range types {
				// Пропускаем исключаемые типы и типы с маршалерами
				// Ищем тип для проверки
				var typ *shared.Type
				for _, t := range r.project.Types {
					if t.TypeName == usage.typeName {
						tPkg := t.ImportPkgPath
						if tPkg == "" {
							tPkg = usage.pkgPath
						}
						if tPkg == usage.pkgPath {
							typ = t
							break
						}
					}
				}
				if typ != nil {
					if r.isExplicitlyExcludedType(typ) || r.hasMarshaler(typ, true) || r.hasMarshaler(typ, false) {
						continue
					}
				}
				typeAnchor := generateAnchor(usage.fullTypeName)
				md.PlainText(fmt.Sprintf("    - [%s](#%s)", usage.fullTypeName, typeAnchor))
				md.LF()
			}
		}
		md.LF()
	}

	// Вспомогательные разделы
	md.PlainText(markdown.Bold("Вспомогательные разделы:"))
	md.LF()
	if hasJsonRPC {
		md.PlainText(fmt.Sprintf("- [Batch запросы (JSON-RPC)](#%s)", generateAnchor("Batch запросы (JSON-RPC)")))
		md.LF()
	}
	md.PlainText(fmt.Sprintf("- [Обработка ошибок](#%s)", generateAnchor("Обработка ошибок")))
	md.LF()
	md.HorizontalRule()

	// Описание клиента
	r.renderClientDescriptionTS(md)

	// Генерируем контракты
	for _, contract := range contracts {
		r.renderContractTS(md, contract, outDir)
	}

	// Генерируем секцию "Общие типы" для всех типов
	if len(allTypes) > 0 {
		r.renderAllTypesTS(md, allTypes)
	}

	// Batch запросы для JSON-RPC
	if hasJsonRPC {
		r.renderBatchSectionTS(md, contracts, outDir)
	}

	// Обработка ошибок
	r.renderErrorsSectionTS(md, outDir)

	if err = md.Build(); err != nil {
		return err
	}

	// Определяем путь к файлу
	outFilename := path.Join(outDir, "README.md")
	if docOpts.FilePath != "" {
		outFilename = docOpts.FilePath
		// Создаём директорию, если её нет
		readmeDir := path.Dir(outFilename)
		if err = os.MkdirAll(readmeDir, 0777); err != nil {
			return err
		}
	}

	return os.WriteFile(outFilename, buf.Bytes(), 0600)
}

// generateAnchor создаёт якорную ссылку из заголовка для Markdown
func generateAnchor(title string) string {
	// Конвертируем в нижний регистр
	anchor := strings.ToLower(title)

	// Заменяем пробелы на дефисы
	anchor = strings.ReplaceAll(anchor, " ", "-")

	// Заменяем / и : на дефисы
	anchor = strings.ReplaceAll(anchor, "/", "-")
	anchor = strings.ReplaceAll(anchor, ":", "-")

	// Заменяем _ на дефисы
	anchor = strings.ReplaceAll(anchor, "_", "-")

	// Удаляем множественные дефисы
	for strings.Contains(anchor, "--") {
		anchor = strings.ReplaceAll(anchor, "--", "-")
	}

	// Удаляем дефисы в начале и конце
	anchor = strings.Trim(anchor, "-")

	if anchor == "" {
		anchor = "section"
	}

	return anchor
}

// methodIsJsonRPC проверяет, является ли метод JSON-RPC методом.
func (r *ClientRenderer) methodIsJsonRPC(contract *shared.Contract, method *shared.Method) bool {
	if method == nil || method.Annotations == nil {
		return false
	}
	// Метод является JSON-RPC, если контракт имеет jsonRPC-server и метод НЕ имеет http-method
	return contract != nil && r.contains(contract.Annotations, TagServerJsonRPC) && !r.contains(method.Annotations, TagMethodHTTP)
}

// methodIsHTTP проверяет, является ли метод HTTP методом.
func (r *ClientRenderer) methodIsHTTP(method *shared.Method) bool {
	if method == nil || method.Annotations == nil {
		return false
	}
	// Проверяем, есть ли аннотация http-method
	return r.contains(method.Annotations, TagMethodHTTP)
}

// filterDocsComments фильтрует аннотации @tg из документации
func filterDocsComments(docs []string) []string {
	if len(docs) == 0 {
		return docs
	}
	var filtered []string
	for _, doc := range docs {
		// Пропускаем строки с аннотациями @tg
		if !strings.Contains(doc, "@tg") {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// renderClientDescriptionTS генерирует общее описание клиента для TypeScript
func (r *ClientRenderer) renderClientDescriptionTS(md *markdown.Markdown) {
	md.H2("Описание клиента")
	md.PlainText("TypeScript клиент для работы с API. Клиент поддерживает JSON-RPC и HTTP методы.")
	md.LF()
	md.PlainText("Основные возможности:")
	md.LF()
	capabilities := []string{
		"Поддержка JSON-RPC 2.0",
		"Поддержка HTTP методов (GET, POST, PUT, DELETE и др.)",
		"Batch запросы для JSON-RPC",
		"Автоматическая обработка ошибок",
		"Типизированные методы для всех контрактов",
	}
	md.BulletList(capabilities...)
	md.LF()

	// Находим первый доступный контракт и метод для примеров
	var exampleContract *shared.Contract
	var exampleMethod *shared.Method

	for _, contract := range r.project.Contracts {
		if r.contains(contract.Annotations, TagServerJsonRPC) {
			for _, method := range contract.Methods {
				if r.methodIsJsonRPC(contract, method) {
					exampleContract = contract
					exampleMethod = method
					break
				}
			}
			if exampleContract != nil {
				break
			}
		}
		if exampleContract == nil && r.contains(contract.Annotations, TagServerHTTP) {
			for _, method := range contract.Methods {
				if r.methodIsHTTP(method) {
					exampleContract = contract
					exampleMethod = method
					break
				}
			}
			if exampleContract != nil {
				break
			}
		}
	}

	// Пример инициализации клиента
	md.PlainText(markdown.Bold("Инициализация клиента:"))
	md.LF()

	if exampleContract != nil && exampleMethod != nil {
		// Используем ту же логику, что и в renderContractClientMethod
		fileName := r.tsFileName(exampleContract)
		parts := strings.Split(fileName, "_")
		serviceVar := ""
		for i, part := range parts {
			if i == 0 {
				serviceVar += part
			} else if len(part) > 0 {
				serviceVar += strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		if r.contains(exampleContract.Annotations, TagServerHTTP) {
			serviceVar += "HTTP"
		}
		args := r.argsWithoutContext(exampleMethod)
		results := r.resultsWithoutError(exampleMethod)

		var methodCall string
		var paramValues []string
		for _, arg := range args {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), exampleContract.PkgPath)
			paramValues = append(paramValues, exampleValue)
		}

		if len(paramValues) > 0 {
			methodCall = fmt.Sprintf("%s.%s(%s)", serviceVar, r.lcName(exampleMethod.Name), strings.Join(paramValues, ", "))
		} else {
			methodCall = fmt.Sprintf("%s.%s()", serviceVar, r.lcName(exampleMethod.Name))
		}

		var resultVar string
		if len(results) > 0 {
			resultVar = "result"
		}

		// Получаем правильное имя метода для получения клиента контракта
		clientMethodName := r.getClientMethodName(exampleContract)

		templateData := map[string]interface{}{
			"ServiceVar":      serviceVar,
			"ClientMethodName": clientMethodName,
			"MethodCall":      methodCall,
		}

		var codeExample string
		var err error
		if resultVar != "" {
			templateData["ResultVar"] = resultVar
			codeExample, err = r.renderTemplate("templates/simple_init_with_result.tmpl", templateData)
		} else {
			codeExample, err = r.renderTemplate("templates/simple_init_no_result.tmpl", templateData)
		}
		if err != nil {
			codeExample = fmt.Sprintf("// Error rendering template: %v", err)
		}
		md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeExample)
	}
	md.LF()

	md.PlainText(markdown.Bold("Инициализация с опциями:"))
	md.LF()

	if exampleContract != nil && exampleMethod != nil {
		// Используем ту же логику, что и в renderContractClientMethod
		fileName := r.tsFileName(exampleContract)
		parts := strings.Split(fileName, "_")
		serviceVar := ""
		for i, part := range parts {
			if i == 0 {
				serviceVar += part
			} else if len(part) > 0 {
				serviceVar += strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		if r.contains(exampleContract.Annotations, TagServerHTTP) {
			serviceVar += "HTTP"
		}
		args := r.argsWithoutContext(exampleMethod)
		results := r.resultsWithoutError(exampleMethod)

		var methodCall string
		var paramValues []string
		for _, arg := range args {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), exampleContract.PkgPath)
			paramValues = append(paramValues, exampleValue)
		}

		if len(paramValues) > 0 {
			methodCall = fmt.Sprintf("%s.%s(%s)", serviceVar, r.lcName(exampleMethod.Name), strings.Join(paramValues, ", "))
		} else {
			methodCall = fmt.Sprintf("%s.%s()", serviceVar, r.lcName(exampleMethod.Name))
		}

		var resultVar string
		if len(results) > 0 {
			resultVar = "result"
		}

		// Получаем правильное имя метода для получения клиента контракта
		clientMethodName := r.getClientMethodName(exampleContract)

		templateData := map[string]interface{}{
			"ServiceVar":      serviceVar,
			"ClientMethodName": clientMethodName,
			"MethodCall":      methodCall,
		}

		var codeExample string
		var err error
		if resultVar != "" {
			templateData["ResultVar"] = resultVar
			codeExample, err = r.renderTemplate("templates/init_with_options_with_result.tmpl", templateData)
		} else {
			codeExample, err = r.renderTemplate("templates/init_with_options_no_result.tmpl", templateData)
		}
		if err != nil {
			codeExample = fmt.Sprintf("// Error rendering template: %v", err)
		}
		md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeExample)
	}
	md.LF()

	// Пример с заголовками
	md.PlainText(markdown.Bold("Инициализация с кастомными заголовками:"))
	md.LF()

	if exampleContract != nil && exampleMethod != nil {
		// Используем ту же логику, что и в renderContractClientMethod
		fileName := r.tsFileName(exampleContract)
		parts := strings.Split(fileName, "_")
		serviceVar := ""
		for i, part := range parts {
			if i == 0 {
				serviceVar += part
			} else if len(part) > 0 {
				serviceVar += strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		if r.contains(exampleContract.Annotations, TagServerHTTP) {
			serviceVar += "HTTP"
		}
		args := r.argsWithoutContext(exampleMethod)
		results := r.resultsWithoutError(exampleMethod)

		var methodCall string
		var paramValues []string
		for _, arg := range args {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), exampleContract.PkgPath)
			paramValues = append(paramValues, exampleValue)
		}

		if len(paramValues) > 0 {
			methodCall = fmt.Sprintf("%s.%s(%s)", serviceVar, r.lcName(exampleMethod.Name), strings.Join(paramValues, ", "))
		} else {
			methodCall = fmt.Sprintf("%s.%s()", serviceVar, r.lcName(exampleMethod.Name))
		}

		var resultVar string
		if len(results) > 0 {
			resultVar = "result"
		}

		// Получаем правильное имя метода для получения клиента контракта
		clientMethodName := r.getClientMethodName(exampleContract)

		templateData := map[string]interface{}{
			"ServiceVar":      serviceVar,
			"ClientMethodName": clientMethodName,
			"MethodName":      exampleMethod.Name,
			"MethodCall":      methodCall,
		}

		var codeExample string
		var err error
		if resultVar != "" {
			templateData["ResultVar"] = resultVar
			codeExample, err = r.renderTemplate("templates/init_with_headers_with_result.tmpl", templateData)
		} else {
			codeExample, err = r.renderTemplate("templates/init_with_headers_no_result.tmpl", templateData)
		}
		if err != nil {
			codeExample = fmt.Sprintf("// Error rendering template: %v", err)
		}
		md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeExample)
	}
	md.LF()

	// Пример с функциональными заголовками
	md.PlainText(markdown.Bold("Инициализация с функциональными заголовками (динамические токены):"))
	md.LF()
	md.PlainText("Для случаев, когда заголовки должны вычисляться при каждом запросе (например, токены авторизации), можно использовать функцию:")
	md.LF()

	if exampleContract != nil && exampleMethod != nil {
		// Используем ту же логику, что и в renderContractClientMethod
		fileName := r.tsFileName(exampleContract)
		parts := strings.Split(fileName, "_")
		serviceVar := ""
		for i, part := range parts {
			if i == 0 {
				serviceVar += part
			} else if len(part) > 0 {
				serviceVar += strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		if r.contains(exampleContract.Annotations, TagServerHTTP) {
			serviceVar += "HTTP"
		}
		args := r.argsWithoutContext(exampleMethod)
		results := r.resultsWithoutError(exampleMethod)

		var methodCall string
		var paramValues []string
		for _, arg := range args {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), exampleContract.PkgPath)
			paramValues = append(paramValues, exampleValue)
		}

		if len(paramValues) > 0 {
			methodCall = fmt.Sprintf("%s.%s(%s)", serviceVar, r.lcName(exampleMethod.Name), strings.Join(paramValues, ", "))
		} else {
			methodCall = fmt.Sprintf("%s.%s()", serviceVar, r.lcName(exampleMethod.Name))
		}

		var resultVar string
		if len(results) > 0 {
			resultVar = "result"
		}

		// Получаем правильное имя метода для получения клиента контракта
		clientMethodName := r.getClientMethodName(exampleContract)

		templateData := map[string]interface{}{
			"ServiceVar":      serviceVar,
			"ClientMethodName": clientMethodName,
			"MethodName":      exampleMethod.Name,
			"MethodCall":      methodCall,
		}

		var codeExample string
		var err error
		if resultVar != "" {
			templateData["ResultVar"] = resultVar
			codeExample, err = r.renderTemplate("templates/init_with_dynamic_headers_with_result.tmpl", templateData)
		} else {
			codeExample, err = r.renderTemplate("templates/init_with_dynamic_headers_no_result.tmpl", templateData)
		}
		if err != nil {
			codeExample = fmt.Sprintf("// Error rendering template: %v", err)
		}
		md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeExample)
	}
	md.LF()
	md.PlainText("Функция заголовков может быть синхронной или асинхронной (возвращать Promise). При каждом запросе функция будет вызвана, что позволяет использовать актуальные токены авторизации.")
	md.LF()
	md.HorizontalRule()
}

// renderContractTS генерирует документацию для контракта
func (r *ClientRenderer) renderContractTS(md *markdown.Markdown, contract *shared.Contract, outDir string) {
	contractAnchor := generateAnchor(contract.Name)
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", contractAnchor))
	md.LF()
	md.H2(contract.Name)

	// Описание контракта из документации
	contractDesc := filterDocsComments(contract.Docs)
	if len(contractDesc) > 0 {
		md.PlainText(strings.Join(contractDesc, "\n"))
		md.LF()
	}

	// JSON-RPC методы
	if r.contains(contract.Annotations, TagServerJsonRPC) {
		for _, method := range contract.Methods {
			if !r.methodIsJsonRPC(contract, method) {
				continue
			}
			r.renderMethodDocTS(md, method, contract, outDir)
		}
	}

	// HTTP методы
	if r.contains(contract.Annotations, TagServerHTTP) {
		for _, method := range contract.Methods {
			if !r.methodIsHTTP(method) {
				continue
			}
			r.renderHTTPMethodDocTS(md, method, contract, outDir)
		}
	}

	md.HorizontalRule()
}

// renderMethodDocTS генерирует документацию для JSON-RPC метода
func (r *ClientRenderer) renderMethodDocTS(md *markdown.Markdown, method *shared.Method, contract *shared.Contract, outDir string) {
	methodAnchor := generateAnchor(method.Name)
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", methodAnchor))
	md.LF()
	md.H3(method.Name)

	// Описание метода
	summary := method.Annotations[tagSummary]
	methodDesc := filterDocsComments(method.Docs)
	desc := ""
	if len(methodDesc) > 0 {
		desc = methodDesc[0]
	}
	if summary == "" && desc != "" {
		summary = desc
	}
	if summary == "" {
		summary = method.Name
	}

	if summary != "" {
		md.PlainText(markdown.Bold("Описание:") + " " + summary)
		md.LF()
	}
	if desc != "" && desc != summary {
		md.PlainText(desc)
		md.LF()
	}

	// Сигнатура метода
	r.renderMethodSignatureTS(md, method, contract, false)

	// Параметры и возвращаемые значения
	r.renderMethodParamsAndResultsTS(md, method, contract)

	// Возможные ошибки
	r.renderMethodErrorsTS(md, method, contract)

	md.LF()
}

// renderHTTPMethodDocTS генерирует документацию для HTTP метода
func (r *ClientRenderer) renderHTTPMethodDocTS(md *markdown.Markdown, method *shared.Method, contract *shared.Contract, outDir string) {
	httpMethod := method.Annotations[TagMethodHTTP]
	if httpMethod == "" {
		httpMethod = "GET"
	}
	httpPath := method.Annotations[TagHttpPath]

	methodTitle := fmt.Sprintf("%s %s", httpMethod, httpPath)
	methodAnchor := generateAnchor(methodTitle)
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", methodAnchor))
	md.LF()
	md.H3(methodTitle)

	// Описание метода
	summary := method.Annotations[tagSummary]
	methodDesc := filterDocsComments(method.Docs)
	desc := ""
	if len(methodDesc) > 0 {
		desc = methodDesc[0]
	}
	if summary == "" && desc != "" {
		summary = desc
	}
	if summary == "" {
		summary = method.Name
	}

	if summary != "" {
		md.PlainText(markdown.Bold("Описание:") + " " + summary)
		md.LF()
	}
	if desc != "" && desc != summary {
		md.PlainText(desc)
		md.LF()
	}

	// Сигнатура метода
	r.renderMethodSignatureTS(md, method, contract, true)

	// Параметры и возвращаемые значения
	r.renderMethodParamsAndResultsTS(md, method, contract)

	// Возможные ошибки
	r.renderMethodErrorsTS(md, method, contract)

	md.LF()
}

// renderMethodSignatureTS генерирует сигнатуру метода в блоке кода
func (r *ClientRenderer) renderMethodSignatureTS(md *markdown.Markdown, method *shared.Method, contract *shared.Contract, isHTTP bool) {
	md.PlainText(markdown.Bold("Сигнатура:"))
	md.LF()

	var sigBuilder strings.Builder
	// Используем ту же логику, что и в renderContractClientMethod
	fileName := r.tsFileName(contract)
	parts := strings.Split(fileName, "_")
	serviceVar := ""
	for i, part := range parts {
		if i == 0 {
			serviceVar += part
		} else if len(part) > 0 {
			serviceVar += strings.ToUpper(string(part[0])) + part[1:]
		}
	}
	if isHTTP {
		serviceVar += "HTTP"
	}
	sigBuilder.WriteString(serviceVar)
	sigBuilder.WriteString(".")
	sigBuilder.WriteString(r.lcName(method.Name))
	sigBuilder.WriteString("(")

	// Параметры
	args := r.argsWithoutContext(method)
	if len(args) > 0 {
		for i, arg := range args {
			if i > 0 {
				sigBuilder.WriteString(", ")
			}
			sigBuilder.WriteString(arg.Name)
			sigBuilder.WriteString(": ")
			typeStr := r.tsTypeStringFromVariable(arg, contract.PkgPath)
			sigBuilder.WriteString(typeStr)
		}
	}

	sigBuilder.WriteString("): Promise<")

	// Возвращаемые значения
	results := r.resultsWithoutError(method)
	if len(results) > 0 {
		if len(results) == 1 {
			typeStr := r.tsTypeStringFromVariable(results[0], contract.PkgPath)
			sigBuilder.WriteString(typeStr)
		} else {
			sigBuilder.WriteString("{ ")
			for i, result := range results {
				if i > 0 {
					sigBuilder.WriteString(", ")
				}
				resultName := result.Name
				if resultName == "" {
					resultName = fmt.Sprintf("result%d", i+1)
				}
				sigBuilder.WriteString(resultName)
				sigBuilder.WriteString(": ")
				typeStr := r.tsTypeStringFromVariable(result, contract.PkgPath)
				sigBuilder.WriteString(typeStr)
			}
			sigBuilder.WriteString(" }")
		}
	} else {
		sigBuilder.WriteString("void")
	}

	sigBuilder.WriteString(">")

	md.CodeBlocks(markdown.SyntaxHighlightTypeScript, sigBuilder.String())
	md.LF()
}

// renderMethodParamsAndResultsTS генерирует таблицы параметров и возвращаемых значений
func (r *ClientRenderer) renderMethodParamsAndResultsTS(md *markdown.Markdown, method *shared.Method, contract *shared.Contract) {
	args := r.argsWithoutContext(method)
	results := r.resultsWithoutError(method)

	// Параметры
	if len(args) > 0 {
		md.PlainText(markdown.Bold("Параметры:"))
		md.LF()

		rows := make([][]string, 0)
		for _, arg := range args {
			typeStr := r.tsTypeStringFromVariable(arg, contract.PkgPath)
			// Ссылка на тип
			typeLink := r.getTypeLinkFromVariableTS(arg, contract.PkgPath)

			rows = append(rows, []string{
				markdown.Code(arg.Name),
				markdown.Code(typeStr),
				typeLink,
			})
		}

		headers := []string{"Имя", "Тип", "Ссылка на тип"}
		tableSet := markdown.TableSet{
			Header: headers,
			Rows:   rows,
		}
		md.Table(tableSet)
		md.LF()
	}

	// Возвращаемые значения
	if len(results) > 0 {
		md.PlainText(markdown.Bold("Возвращаемые значения:"))
		md.LF()

		rows := make([][]string, 0)
		for _, result := range results {
			typeStr := r.tsTypeStringFromVariable(result, contract.PkgPath)
			// Ссылка на тип
			typeLink := r.getTypeLinkFromVariableTS(result, contract.PkgPath)

			resultName := result.Name
			if resultName == "" {
				resultName = "result"
			}

			rows = append(rows, []string{
				markdown.Code(resultName),
				markdown.Code(typeStr),
				typeLink,
			})
		}

		headers := []string{"Имя", "Тип", "Ссылка на тип"}
		tableSet := markdown.TableSet{
			Header: headers,
			Rows:   rows,
		}
		md.Table(tableSet)
		md.LF()
	}
}

// renderMethodErrorsTS генерирует описание возможных ошибок метода
func (r *ClientRenderer) renderMethodErrorsTS(md *markdown.Markdown, method *shared.Method, contract *shared.Contract) {
	// Используем информацию об ошибках из метода
	if len(method.Errors) == 0 {
		return
	}

	md.LF()
	md.PlainText(markdown.Bold("Возможные ошибки:"))
	md.LF()

	// Сортируем ошибки по HTTP коду для детерминированного порядка
	errors := make([]*shared.ErrorInfo, len(method.Errors))
	copy(errors, method.Errors)
	sort.Slice(errors, func(i, j int) bool {
		// Сначала ошибки с HTTP кодом, затем без
		if errors[i].HTTPCode == 0 && errors[j].HTTPCode != 0 {
			return false
		}
		if errors[i].HTTPCode != 0 && errors[j].HTTPCode == 0 {
			return true
		}
		// Если оба с кодом или оба без - сортируем по коду
		return errors[i].HTTPCode < errors[j].HTTPCode
	})

	// Описываем все ошибки
	for _, errInfo := range errors {
		// Заголовок ошибки
		if errInfo.HTTPCode != 0 {
			errorDesc := fmt.Sprintf("%s (%d)", errInfo.HTTPCodeText, errInfo.HTTPCode)
			md.PlainText(fmt.Sprintf("- %s - %s", markdown.Code(fmt.Sprintf("%d", errInfo.HTTPCode)), errorDesc))
		} else {
			md.PlainText(fmt.Sprintf("- %s", markdown.Code(errInfo.TypeName)))
		}
		md.LF()
	}
}

// renderBatchSectionTS генерирует секцию Batch запросов для JSON-RPC
func (r *ClientRenderer) renderBatchSectionTS(md *markdown.Markdown, contracts []*shared.Contract, outDir string) {
	batchAnchor := generateAnchor("Batch запросы (JSON-RPC)")
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", batchAnchor))
	md.LF()
	md.H2("Batch запросы (JSON-RPC)")
	md.PlainText("Для выполнения нескольких JSON-RPC запросов одновременно используйте метод " + markdown.Code("batch") + ". Это позволяет отправить несколько запросов в одном HTTP запросе.")
	md.LF()

	md.PlainText(markdown.Bold("Пример использования:"))
	md.LF()

	r.renderBatchExampleTS(md, contracts, outDir)
	md.LF()

	md.HorizontalRule()
}

// renderBatchExampleTS генерирует пример использования batch для TypeScript
func (r *ClientRenderer) renderBatchExampleTS(md *markdown.Markdown, contracts []*shared.Contract, outDir string) {
	// Находим JSON-RPC контракты и методы для примеров
	var jsonRPCContracts []*shared.Contract
	for _, contract := range contracts {
		if r.contains(contract.Annotations, TagServerJsonRPC) {
			hasJsonRPCMethods := false
			for _, method := range contract.Methods {
				if r.methodIsJsonRPC(contract, method) {
					hasJsonRPCMethods = true
					break
				}
			}
			if hasJsonRPCMethods {
				jsonRPCContracts = append(jsonRPCContracts, contract)
			}
		}
	}

	if len(jsonRPCContracts) == 0 {
		// Нет JSON-RPC контрактов - используем простой пример
		codeExample := `import { Client } from './client';

const client = new Client('http://localhost:9000');

await client.batch(
  service.reqMethod1(callback1, param1, param2),
  service.reqMethod2(callback2, param3)
);`
		md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeExample)
		return
	}

	// Используем первые два метода из разных контрактов или один контракт
	exampleContract := jsonRPCContracts[0]
	var exampleMethod1 *shared.Method
	var exampleMethod2 *shared.Method
	for _, method := range exampleContract.Methods {
		if r.methodIsJsonRPC(exampleContract, method) {
			if exampleMethod1 == nil {
				exampleMethod1 = method
			} else {
				exampleMethod2 = method
				break
			}
		}
	}

	// Если нет второго метода, используем метод из другого контракта
	if exampleMethod2 == nil && len(jsonRPCContracts) > 1 {
		for _, contract := range jsonRPCContracts[1:] {
			for _, method := range contract.Methods {
				if r.methodIsJsonRPC(contract, method) {
					exampleMethod2 = method
					break
				}
			}
			if exampleMethod2 != nil {
				break
			}
		}
	}

	// Генерируем пример
	var codeBuilder strings.Builder
	codeBuilder.WriteString("import { Client } from './client';\n\n")
	codeBuilder.WriteString("const client = new Client('http://localhost:9000');\n\n")

	// Получаем имя сервиса
	serviceVar := r.getClientMethodName(exampleContract)
	codeBuilder.WriteString(fmt.Sprintf("const %s = client.%s();\n\n", serviceVar, serviceVar))
	codeBuilder.WriteString("// Создаем batch запрос с несколькими методами\n")
	codeBuilder.WriteString("const requests = [\n")

	// Первый метод
	if exampleMethod1 != nil {
		args1 := r.argsWithoutContext(exampleMethod1)
		results1 := r.resultsWithoutError(exampleMethod1)
		callbackName1 := "callback1"
		
		var paramValues1 []string
		for _, arg := range args1 {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), exampleContract.PkgPath)
			paramValues1 = append(paramValues1, exampleValue)
		}
		
		reqCall1 := fmt.Sprintf("  %s.req%s(%s", serviceVar, exampleMethod1.Name, callbackName1)
		if len(paramValues1) > 0 {
			reqCall1 += ", " + strings.Join(paramValues1, ", ")
		}
		reqCall1 += "),"
		codeBuilder.WriteString(reqCall1 + "\n")

		// Callback функция
		codeBuilder.WriteString("\n")
		callbackParams1 := make([]string, 0)
		for _, ret := range results1 {
			typeStr := r.walkVariable(ret.Name, exampleContract.PkgPath, ret, exampleMethod1.Annotations, false).typeLink()
			callbackParams1 = append(callbackParams1, fmt.Sprintf("%s: %s", ret.Name, typeStr))
		}
		callbackParams1 = append(callbackParams1, "error: Error | null")
		codeBuilder.WriteString(fmt.Sprintf("function %s(%s) {\n", callbackName1, strings.Join(callbackParams1, ", ")))
		codeBuilder.WriteString("  if (error) {\n")
		codeBuilder.WriteString(fmt.Sprintf("    console.error('Error in %s.%s:', error);\n", exampleContract.Name, exampleMethod1.Name))
		codeBuilder.WriteString("    return;\n")
		codeBuilder.WriteString("  }\n")
		if len(results1) > 0 {
			resultVar := results1[0].Name
			codeBuilder.WriteString(fmt.Sprintf("  console.log('Result:', %s);\n", resultVar))
		}
		codeBuilder.WriteString("}\n")
	}

	// Второй метод
	if exampleMethod2 != nil {
		args2 := r.argsWithoutContext(exampleMethod2)
		results2 := r.resultsWithoutError(exampleMethod2)
		callbackName2 := "callback2"
		
		var paramValues2 []string
		for _, arg := range args2 {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), exampleContract.PkgPath)
			paramValues2 = append(paramValues2, exampleValue)
		}
		
		reqCall2 := fmt.Sprintf("  %s.req%s(%s", serviceVar, exampleMethod2.Name, callbackName2)
		if len(paramValues2) > 0 {
			reqCall2 += ", " + strings.Join(paramValues2, ", ")
		}
		reqCall2 += "),"
		codeBuilder.WriteString(reqCall2 + "\n")

		// Callback функция
		codeBuilder.WriteString("\n")
		callbackParams2 := make([]string, 0)
		for _, ret := range results2 {
			typeStr := r.walkVariable(ret.Name, exampleContract.PkgPath, ret, exampleMethod2.Annotations, false).typeLink()
			callbackParams2 = append(callbackParams2, fmt.Sprintf("%s: %s", ret.Name, typeStr))
		}
		callbackParams2 = append(callbackParams2, "error: Error | null")
		codeBuilder.WriteString(fmt.Sprintf("function %s(%s) {\n", callbackName2, strings.Join(callbackParams2, ", ")))
		codeBuilder.WriteString("  if (error) {\n")
		codeBuilder.WriteString(fmt.Sprintf("    console.error('Error in %s.%s:', error);\n", exampleContract.Name, exampleMethod2.Name))
		codeBuilder.WriteString("    return;\n")
		codeBuilder.WriteString("  }\n")
		if len(results2) > 0 {
			resultVar := results2[0].Name
			codeBuilder.WriteString(fmt.Sprintf("  console.log('Result:', %s);\n", resultVar))
		}
		codeBuilder.WriteString("}\n")
	}

	codeBuilder.WriteString("];\n\n")
	codeBuilder.WriteString("// Выполняем batch запрос\n")
	codeBuilder.WriteString("await client.batch(requests);\n\n")
	codeBuilder.WriteString("// Callback функции будут вызваны автоматически при получении ответов\n")

	md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeBuilder.String())
}

// renderErrorsSectionTS генерирует секцию обработки ошибок
func (r *ClientRenderer) renderErrorsSectionTS(md *markdown.Markdown, outDir string) {
	errorsAnchor := generateAnchor("Обработка ошибок")
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", errorsAnchor))
	md.LF()
	md.H2("Обработка ошибок")

	// Находим первый JSON-RPC контракт и метод для примера
	var jsonrpcContract *shared.Contract
	var jsonrpcMethod *shared.Method
	for _, contract := range r.project.Contracts {
		if r.contains(contract.Annotations, TagServerJsonRPC) {
			for _, method := range contract.Methods {
				if r.methodIsJsonRPC(contract, method) {
					jsonrpcContract = contract
					jsonrpcMethod = method
					break
				}
			}
			if jsonrpcContract != nil {
				break
			}
		}
	}

	jsonrpcErrorsAnchor := generateAnchor("JSON-RPC ошибки")
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", jsonrpcErrorsAnchor))
	md.LF()
	md.H3("JSON-RPC ошибки")
	md.PlainText("При работе с JSON-RPC клиентом ошибки обрабатываются автоматически. Если сервер возвращает ошибку в формате JSON-RPC 2.0, клиент выбрасывает исключение.")

	var codeExample string
	if jsonrpcContract != nil && jsonrpcMethod != nil {
		// Используем ту же логику, что и в renderContractClientMethod
		fileName := r.tsFileName(jsonrpcContract)
		parts := strings.Split(fileName, "_")
		serviceVar := ""
		for i, part := range parts {
			if i == 0 {
				serviceVar += part
			} else if len(part) > 0 {
				serviceVar += strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		clientMethodName := r.getClientMethodName(jsonrpcContract)
		methodName := r.lcName(jsonrpcMethod.Name)
		args := r.argsWithoutContext(jsonrpcMethod)
		
		var paramValues []string
		for _, arg := range args {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), jsonrpcContract.PkgPath)
			paramValues = append(paramValues, exampleValue)
		}
		
		var methodCall string
		if len(paramValues) > 0 {
			methodCall = fmt.Sprintf("%s.%s(%s)", serviceVar, methodName, strings.Join(paramValues, ", "))
		} else {
			methodCall = fmt.Sprintf("%s.%s()", serviceVar, methodName)
		}
		
		codeExample = fmt.Sprintf(`import { Client } from './client';

const client = new Client('http://localhost:9000');
const %s = client.%s();

try {
  const result = await %s;
  // Используем result
} catch (error) {
  // Обрабатываем ошибку
  console.error('Error:', error);
}`, serviceVar, clientMethodName, methodCall)
	} else {
		codeExample = `import { Client } from './client';

const client = new Client('http://localhost:9000');
const service = client.jsonRpcService();

try {
  const result = await service.someMethod({ param: 'value' });
  // Используем result
} catch (error) {
  // Обрабатываем ошибку
  console.error('Error:', error);
}`
	}

	md.CodeBlocks(markdown.SyntaxHighlightTypeScript, codeExample)

	md.PlainText(markdown.Bold("Коды ошибок JSON-RPC 2.0:"))
	errorCodes := []string{
		markdown.Code("-32700") + " - Parse error",
		markdown.Code("-32600") + " - Invalid Request",
		markdown.Code("-32601") + " - Method not found",
		markdown.Code("-32602") + " - Invalid params",
		markdown.Code("-32603") + " - Internal error",
		markdown.Code("-32000 to -32099") + " - Server error (зарезервировано для серверных ошибок)",
	}
	md.BulletList(errorCodes...)

	// Находим первый HTTP контракт и метод для примера
	var httpContract *shared.Contract
	var httpMethod *shared.Method
	for _, contract := range r.project.Contracts {
		if r.contains(contract.Annotations, TagServerHTTP) {
			for _, method := range contract.Methods {
				if r.methodIsHTTP(method) {
					httpContract = contract
					httpMethod = method
					break
				}
			}
			if httpContract != nil {
				break
			}
		}
	}

	httpErrorsAnchor := generateAnchor("HTTP ошибки")
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", httpErrorsAnchor))
	md.LF()
	md.H3("HTTP ошибки")
	md.PlainText("При работе с HTTP клиентом ошибки обрабатываются автоматически. Если сервер возвращает HTTP статус код, отличный от ожидаемого успешного кода, клиент выбрасывает исключение.")

	var httpCodeExample string
	if httpContract != nil && httpMethod != nil {
		// Используем ту же логику, что и в renderContractClientMethod
		fileName := r.tsFileName(httpContract)
		parts := strings.Split(fileName, "_")
		serviceVar := ""
		for i, part := range parts {
			if i == 0 {
				serviceVar += part
			} else if len(part) > 0 {
				serviceVar += strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		serviceVar += "HTTP"
		clientMethodName := r.getClientMethodName(httpContract)
		methodName := r.lcName(httpMethod.Name)
		args := r.argsWithoutContext(httpMethod)
		
		var paramValues []string
		for _, arg := range args {
			exampleValue := r.generateExampleValueFromVariable(arg, strings.Join(arg.Docs, "\n"), httpContract.PkgPath)
			paramValues = append(paramValues, exampleValue)
		}
		
		var methodCall string
		if len(paramValues) > 0 {
			methodCall = fmt.Sprintf("%s.%s(%s)", serviceVar, methodName, strings.Join(paramValues, ", "))
		} else {
			methodCall = fmt.Sprintf("%s.%s()", serviceVar, methodName)
		}
		
		httpCodeExample = fmt.Sprintf(`import { Client } from './client';

const client = new Client('http://localhost:9000');
const %s = client.%s();

try {
  const result = await %s;
  // Используем result
} catch (error) {
  // Обрабатываем ошибку
  console.error('HTTP Error:', error);
}`, serviceVar, clientMethodName, methodCall)
	} else {
		httpCodeExample = `import { Client } from './client';

const client = new Client('http://localhost:9000');
const httpService = client.httpServiceHTTP();

try {
  const result = await httpService.getItem({ id: '123' });
  // Используем result
} catch (error) {
  // Обрабатываем ошибку
  console.error('HTTP Error:', error);
}`
	}

	md.CodeBlocks(markdown.SyntaxHighlightTypeScript, httpCodeExample)

	md.PlainText("HTTP клиент автоматически проверяет статус код ответа и выбрасывает исключение, если код не соответствует ожидаемому успешному коду для метода.")
	md.LF()
	md.HorizontalRule()
}

// tsTypeStringFromVariable возвращает строковое представление TypeScript типа из Variable
func (r *ClientRenderer) tsTypeStringFromVariable(variable *shared.Variable, pkgPath string) string {
	schema := r.walkVariable(variable.Name, pkgPath, variable, nil, false)
	return schema.typeLink()
}

// renderTemplate рендерит шаблон из embed FS
func (r *ClientRenderer) renderTemplate(templatePath string, data interface{}) (string, error) {
	contentBytes, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
	}).Parse(string(contentBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return buf.String(), nil
}

// generateExampleValueFromVariable генерирует пример значения для переменной в TypeScript
func (r *ClientRenderer) generateExampleValueFromVariable(variable *shared.Variable, docs, pkgPath string) string {
	// Обрабатываем массивы и слайсы
	if variable.IsSlice || variable.ArrayLen > 0 {
		return "[]"
	}

	// Обрабатываем map
	if variable.MapKeyID != "" && variable.MapValueID != "" {
		return "{}"
	}

	// Базовый тип - используем простые примеры
	typeStr := r.tsTypeStringFromVariable(variable, pkgPath)
	switch typeStr {
	case "string":
		return `"example"`
	case "number", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
		return "0"
	case "boolean", "bool":
		return "false"
	case "Date":
		return "new Date()"
	default:
		// Для сложных типов возвращаем пустой объект или null
		if variable.NumberOfPointers > 0 || strings.Contains(typeStr, "| null") || strings.Contains(typeStr, "| undefined") {
			return "null"
		}
		// Если это объект или интерфейс, возвращаем пустой объект
		if strings.Contains(typeStr, "{") || strings.Contains(typeStr, "Record") {
			return "{}"
		}
		return fmt.Sprintf("{} as %s", typeStr)
	}
}

// getClientMethodName возвращает правильное имя метода для получения клиента контракта
func (r *ClientRenderer) getClientMethodName(contract *shared.Contract) string {
	fileName := r.tsFileName(contract)
	// Преобразуем snake_case в lowerCamelCase для имени метода
	// Например: "http_service" -> "httpService", "json_rpc_service" -> "jsonRpcService"
	methodName := ""
	parts := strings.Split(fileName, "_")
	for i, part := range parts {
		if i == 0 {
			methodName += part
		} else if len(part) > 0 {
			methodName += strings.ToUpper(string(part[0])) + part[1:]
		}
	}
	
	// Для HTTP контрактов добавляем суффикс HTTP
	if r.contains(contract.Annotations, TagServerHTTP) {
		methodName += "HTTP"
	}
	
	return methodName
}

// getStructType возвращает структуру типа по typeID
func (r *ClientRenderer) getStructType(typeID, pkgPath string) (structType *shared.Type, typeName string, pkg string) {
	typ, ok := r.project.Types[typeID]
	if !ok {
		return nil, "", ""
	}

	// Проверяем, является ли тип структурой
	if typ.Kind != shared.TypeKindStruct || typ.TypeName == "" {
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

// getTypeLinkFromVariableTS возвращает ссылку на тип для переменной (параметр или возвращаемое значение) в TypeScript
func (r *ClientRenderer) getTypeLinkFromVariableTS(variable *shared.Variable, pkgPath string) string {
	switch {
	case variable.IsSlice || variable.ArrayLen > 0:
		// Для массивов/слайсов - ссылка на тип элемента (без [])
		return r.getTypeLinkTS(variable.TypeID, pkgPath)
	case variable.MapKeyID != "" && variable.MapValueID != "":
		// Для map - две ссылки: на ключ и значение (примитивы пропускаем)
		keyLink := r.getTypeLinkTS(variable.MapKeyID, pkgPath)
		valueLink := r.getTypeLinkTS(variable.MapValueID, pkgPath)
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
		return r.getTypeLinkTS(variable.TypeID, pkgPath)
	}
}

// getTypeLinkTS возвращает ссылку на тип или "-" если это примитив
func (r *ClientRenderer) getTypeLinkTS(typeID, pkgPath string) string {
	// Проверяем, является ли тип встроенным примитивом
	if r.isBuiltinType(typeID) {
		return "-"
	}

	// Получаем тип для проверки
	typ, ok := r.project.Types[typeID]
	if !ok {
		return "-"
	}
	
	// Проверяем, является ли тип исключаемым (time.Time и т.д.)
	if r.isExplicitlyExcludedType(typ) {
		return "-" // Встроенный тип TypeScript, без ссылки
	}
	
	// Проверяем наличие маршалера - типы с маршалерами конвертируются в any
	// Для ссылок проверяем оба маршалера (тип может использоваться и в запросах, и в ответах)
	if r.hasMarshaler(typ, true) || r.hasMarshaler(typ, false) {
		return "-" // any - встроенный тип TypeScript, без ссылки
	}

	// Проверяем, является ли тип структурой
	if structType, _, typePkg := r.getStructType(typeID, pkgPath); structType != nil {
		// Это структура - создаём ссылку на таблицу типа
		// Получаем строковое представление типа для отображения (с namespace)
		typeStr := r.tsTypeString(typeID, pkgPath)
		
		// Для создания якоря всегда используем полное имя с namespace
		// В разделе "Общие типы" все типы отображаются с namespace (например, "dto.SomeStruct")
		// Поэтому якорь должен соответствовать полному имени типа с namespace
		typ, ok := r.project.Types[typeID]
		if !ok {
			return "-"
		}
		
		// Проверяем, является ли тип исключаемым (time.Time и т.д.)
		if r.isExplicitlyExcludedType(typ) {
			return "-" // Встроенный тип TypeScript, без ссылки
		}
		
		// Проверяем наличие маршалера - типы с маршалерами конвертируются в any
		// Для ссылок проверяем оба маршалера (тип может использоваться и в запросах, и в ответах)
		if r.hasMarshaler(typ, true) || r.hasMarshaler(typ, false) {
			return "-" // any - встроенный тип TypeScript, без ссылки
		}
		
		// Определяем namespace для типа
		namespace := ""
		switch {
		case typ.PkgName != "":
			namespace = typ.PkgName
		case typ.ImportAlias != "":
			namespace = typ.ImportAlias
		case typ.ImportPkgPath != "":
			parts := strings.Split(typ.ImportPkgPath, "/")
			if len(parts) > 0 {
				namespace = parts[len(parts)-1]
			}
		case typePkg != "":
			parts := strings.Split(typePkg, "/")
			if len(parts) > 0 {
				namespace = parts[len(parts)-1]
			}
		}
		
		// Формируем полное имя типа для якоря (с namespace)
		typeNameForAnchor := typ.TypeName
		if typeNameForAnchor == "" {
			typeNameForAnchor = typeStr
		}
		fullTypeNameForAnchor := typeNameForAnchor
		if namespace != "" {
			fullTypeNameForAnchor = fmt.Sprintf("%s.%s", namespace, typeNameForAnchor)
		}
		
		// Якорь должен включать namespace (например, "dto.SomeStruct" -> "#dto.somestruct")
		typeAnchor := generateAnchor(fullTypeNameForAnchor)
		return fmt.Sprintf("[%s](#%s)", typeStr, typeAnchor)
	}

	// Проверяем, является ли тип внешним
	typ3, ok := r.project.Types[typeID]
	if ok && typ3.ImportPkgPath != "" && !r.isTypeFromCurrentProject(typ3.ImportPkgPath) {
		// Проверяем, является ли тип исключаемым (time.Time и т.д.)
		if r.isExplicitlyExcludedType(typ3) {
			return "-" // Встроенный тип TypeScript, без ссылки
		}
		// Проверяем наличие маршалера - типы с маршалерами конвертируются в any
		if r.hasMarshaler(typ3, true) || r.hasMarshaler(typ3, false) {
			return "-" // any - встроенный тип TypeScript, без ссылки
		}
		// Внешний тип - для TypeScript обычно не создаём ссылки на внешние библиотеки
		return "-"
	}

	// Локальный тип или примитив - без ссылки
	return "-"
}

// tsTypeString возвращает строковое представление TypeScript типа из typeID
func (r *ClientRenderer) tsTypeString(typeID, pkgPath string) string {
	// Проверяем встроенные типы
	if r.isBuiltinType(typeID) {
		return r.typeIDToTSType(typeID)
	}

	// Получаем тип из project.Types
	typ, ok := r.project.Types[typeID]
	if !ok {
		return r.typeIDToTSType(typeID)
	}

	// Проверяем, является ли тип исключаемым (time.Time и т.д.)
	if r.isExplicitlyExcludedType(typ) {
		// time.Time -> Date
		if typ.ImportPkgPath == "time" && typ.TypeName == "Time" {
			return "Date"
		}
		// Для других исключаемых типов используем typeIDToTSType
		return r.typeIDToTSType(typeID)
	}
	
	// Проверяем наличие маршалера - типы с маршалерами конвертируются в any
	// Для отображения типа в README проверяем оба маршалера (тип может использоваться и в запросах, и в ответах)
	if r.hasMarshaler(typ, true) || r.hasMarshaler(typ, false) {
		return "any"
	}

	// Формируем имя типа
	typeName := typ.TypeName
	if typeName == "" {
		typeName = typeID
	}

	// Если тип из другого пакета, добавляем namespace
	if typ.ImportPkgPath != "" && typ.ImportPkgPath != pkgPath {
		// Используем PkgName или ImportAlias для namespace
		namespace := ""
		switch {
		case typ.PkgName != "":
			namespace = typ.PkgName
		case typ.ImportAlias != "":
			namespace = typ.ImportAlias
		default:
			// Извлекаем имя пакета из пути
			parts := strings.Split(typ.ImportPkgPath, "/")
			if len(parts) > 0 {
				namespace = parts[len(parts)-1]
			}
		}
		if namespace != "" {
			return fmt.Sprintf("%s.%s", namespace, castTypeTs(typeName))
		}
	}

	return castTypeTs(typeName)
}

// typeUsageTS содержит информацию об использовании типа для TypeScript
type typeUsageTS struct {
	typeName     string
	pkgPath      string
	fullTypeName string
	locations    []string
}

// collectStructTypesTS собирает все используемые типы структур для TypeScript
func (r *ClientRenderer) collectStructTypesTS() map[string]*typeUsageTS {
	typeUsages := make(map[string]*typeUsageTS)

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

					// Формируем полное имя типа для отображения (с namespace если нужно)
					fullTypeNameForKey := r.tsTypeString(arg.TypeID, contract.PkgPath)
					if fullTypeNameForKey == "" {
						fullTypeNameForKey = typeName
					}

					if _, ok := typeUsages[key]; !ok {
						typeUsages[key] = &typeUsageTS{
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
				// Пропускаем типы с маршалерами - в TypeScript они конвертируются в any
				typ, ok := r.project.Types[result.TypeID]
				if ok {
					// Проверяем, является ли тип исключаемым (time.Time и т.д.)
					if r.isExplicitlyExcludedType(typ) {
						continue
					}
					// Проверяем наличие маршалера для результатов (Unmarshaler)
					if r.hasMarshaler(typ, false) {
						continue
					}
				}
				
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

					// Формируем полное имя типа для отображения (с namespace если нужно)
					fullTypeNameForKey := r.tsTypeString(result.TypeID, contract.PkgPath)
					if fullTypeNameForKey == "" {
						fullTypeNameForKey = typeName
					}

					if _, ok := typeUsages[key]; !ok {
						typeUsages[key] = &typeUsageTS{
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

// renderAllTypesTS генерирует секцию "Общие типы" для всех типов, используемых в контрактах
func (r *ClientRenderer) renderAllTypesTS(md *markdown.Markdown, allTypes map[string]*typeUsageTS) {
	sharedTypesAnchor := generateAnchor("Общие типы")
	md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", sharedTypesAnchor))
	md.LF()
	md.H2("Общие типы")
	md.PlainText("Типы данных, используемые в клиенте. Типы, используемые в нескольких методах, описаны здесь для избежания дублирования.")
	md.LF()

	// Группируем типы по namespace
	typesByNamespace := make(map[string][]*typeUsageTS)
	for _, usage := range allTypes {
		// Извлекаем namespace из fullTypeName (например, "dto.SomeStruct" -> "dto")
		namespace := ""
		if strings.Contains(usage.fullTypeName, ".") {
			parts := strings.SplitN(usage.fullTypeName, ".", 2)
			namespace = parts[0]
		}
		
		if typesByNamespace[namespace] == nil {
			typesByNamespace[namespace] = make([]*typeUsageTS, 0)
		}
		typesByNamespace[namespace] = append(typesByNamespace[namespace], usage)
	}

	// Сортируем namespace
	namespaceKeys := make([]string, 0, len(typesByNamespace))
	for ns := range typesByNamespace {
		namespaceKeys = append(namespaceKeys, ns)
	}
	sort.Strings(namespaceKeys)

	// Рендерим типы по namespace
	for _, namespace := range namespaceKeys {
		types := typesByNamespace[namespace]
		
		// Сортируем типы внутри namespace
		sort.Slice(types, func(i, j int) bool {
			return types[i].fullTypeName < types[j].fullTypeName
		})

		// Если есть namespace, создаём подзаголовок
		// Пропускаем namespace "time" - в TypeScript time.Time конвертируется в Date
		if namespace != "" && namespace != "time" {
			namespaceAnchor := generateAnchor(namespace)
			md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", namespaceAnchor))
			md.LF()
			md.H3(namespace)
			md.LF()
		}

		for _, usage := range types {
			// Получаем структуру типа - ищем по pkgPath и typeName
			var typ *shared.Type
			for _, t := range r.project.Types {
				if t.TypeName == usage.typeName {
					// Проверяем соответствие pkgPath
					tPkg := t.ImportPkgPath
					if tPkg == "" {
						tPkg = usage.pkgPath
					}
					if tPkg == usage.pkgPath {
						typ = t
						break
					}
				}
			}

			// Пропускаем типы с маршалерами и исключаемые типы - в TypeScript они конвертируются в встроенные типы
			if typ != nil {
				// Проверяем, является ли тип исключаемым (time.Time и т.д.)
				if r.isExplicitlyExcludedType(typ) {
					continue
				}
				// Для типов в разделе "Общие типы" проверяем оба маршалера (могут использоваться и в запросах, и в ответах)
				// Если тип имеет хотя бы один маршалер, пропускаем его
				if r.hasMarshaler(typ, true) || r.hasMarshaler(typ, false) {
					continue
				}
			}

			// Проверяем, является ли тип из внешней библиотеки
			isExternal := typ != nil && typ.ImportPkgPath != "" && !r.isTypeFromCurrentProject(typ.ImportPkgPath)
			
			// В TypeScript все типы должны быть локальными (внешние типы конвертируются в встроенные)
			// Поэтому не выводим информацию о внешних библиотеках
			if typ != nil && typ.Kind == shared.TypeKindStruct && !isExternal {
				// Локальная структура - выводим таблицу полей
				r.renderStructTypeTableTS(md, typ, usage.fullTypeName, usage.pkgPath)
			} else if typ != nil && !isExternal {
				// Если тип не структура, но локальный - выводим только заголовок
				typeAnchor := generateAnchor(usage.fullTypeName)
				md.PlainText(fmt.Sprintf("<a id=\"%s\"></a>", typeAnchor))
				md.LF()
				md.H4(usage.fullTypeName)
				md.LF()
			}
			// Пропускаем внешние типы - в TypeScript они конвертируются в встроенные типы
		}
	}

	md.HorizontalRule()
}

// renderStructTypeTableTS генерирует таблицу для типа структуры в TypeScript
// Вызывается только для локальных типов (не из внешних библиотек)
func (r *ClientRenderer) renderStructTypeTableTS(md *markdown.Markdown, structType *shared.Type, typeName string, pkgPath string) {
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
		typeStr := r.tsTypeStringFromStructField(field, pkgPath)

		// Required
		fieldTags := parseTagsFromDocs(field.Docs)
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
		typeLink := r.getTypeLinkFromStructFieldTS(field, pkgPath)

		// Формируем строку таблицы
		rows = append(rows, []string{
			markdown.Code(fieldName),
			markdown.Code(typeStr),
			requiredStr,
			nullableStr,
			omitemptyStr,
			typeLink,
		})
	}

	// Формируем заголовки
	headers := []string{"Поле", "Тип", "Обязательное", "Nullable", "Omitempty", "Ссылка на тип"}

	tableSet := markdown.TableSet{
		Header: headers,
		Rows:   rows,
	}
	md.Table(tableSet)
	md.LF()
}

// tsTypeStringFromStructField возвращает строковое представление TypeScript типа из StructField
func (r *ClientRenderer) tsTypeStringFromStructField(field *shared.StructField, pkgPath string) string {
	// Обрабатываем массивы и слайсы
	if field.IsSlice || field.ArrayLen > 0 {
		elemType := r.tsTypeString(field.TypeID, pkgPath)
		if field.IsSlice {
			return fmt.Sprintf("%s[]", elemType)
		}
		return fmt.Sprintf("%s[]", elemType) // Для TypeScript массивы фиксированного размера тоже []
	}

	// Обрабатываем map
	if field.MapKeyID != "" && field.MapValueID != "" {
		keyType := r.tsTypeString(field.MapKeyID, pkgPath)
		valueType := r.tsTypeString(field.MapValueID, pkgPath)
		return fmt.Sprintf("Record<%s, %s>", keyType, valueType)
	}

	// Базовый тип
	return r.tsTypeString(field.TypeID, pkgPath)
}

// getTypeLinkFromStructFieldTS возвращает ссылку на тип для поля структуры в TypeScript
func (r *ClientRenderer) getTypeLinkFromStructFieldTS(field *shared.StructField, pkgPath string) string {
	switch {
	case field.IsSlice || field.ArrayLen > 0:
		// Для массивов/слайсов - ссылка на тип элемента (без [])
		return r.getTypeLinkTS(field.TypeID, pkgPath)
	case field.MapKeyID != "" && field.MapValueID != "":
		// Для map - две ссылки: на ключ и значение (примитивы пропускаем)
		keyLink := r.getTypeLinkTS(field.MapKeyID, pkgPath)
		valueLink := r.getTypeLinkTS(field.MapValueID, pkgPath)
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
		return r.getTypeLinkTS(field.TypeID, pkgPath)
	}
}


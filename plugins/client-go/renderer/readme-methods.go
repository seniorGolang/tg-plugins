// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"sort"
	"strings"

	"tgp/internal/markdown"

	"tgp/core"
)

// renderMethodDoc генерирует документацию для JSON-RPC метода
func (r *ClientRenderer) renderMethodDoc(md *markdown.Markdown, method *core.Method, contract *core.Contract, outDir string, typeUsages map[string]*typeUsage) {
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
	r.renderMethodSignature(md, method, contract, outDir, false)

	// Параметры и возвращаемые значения
	r.renderMethodParamsAndResults(md, method, contract, typeUsages)

	// Возможные ошибки
	r.renderMethodErrors(md, method, contract)

	md.LF()
}

// renderHTTPMethodDoc генерирует документацию для HTTP метода
func (r *ClientRenderer) renderHTTPMethodDoc(md *markdown.Markdown, method *core.Method, contract *core.Contract, outDir string, typeUsages map[string]*typeUsage) {
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
	r.renderMethodSignature(md, method, contract, outDir, true)

	// Параметры и возвращаемые значения
	r.renderMethodParamsAndResults(md, method, contract, typeUsages)

	// Возможные ошибки
	r.renderMethodErrors(md, method, contract)

	md.LF()
}

// renderMethodSignature генерирует сигнатуру метода в блоке кода
func (r *ClientRenderer) renderMethodSignature(md *markdown.Markdown, method *core.Method, contract *core.Contract, outDir string, isHTTP bool) {
	md.PlainText(markdown.Bold("Сигнатура:"))
	md.LF()

	var sigBuilder strings.Builder
	sigBuilder.WriteString("func (s *Client")
	if isHTTP {
		sigBuilder.WriteString(contract.Name + "HTTP")
	} else {
		sigBuilder.WriteString(contract.Name)
	}
	sigBuilder.WriteString(") ")
	sigBuilder.WriteString(method.Name)
	sigBuilder.WriteString("(ctx context.Context")

	// Параметры
	args := r.argsWithoutContext(method)
	for _, arg := range args {
		sigBuilder.WriteString(", ")
		sigBuilder.WriteString(arg.Name)
		sigBuilder.WriteString(" ")
		typeStr := r.goTypeStringFromVariable(arg, contract.PkgPath)
		sigBuilder.WriteString(typeStr)
	}

	sigBuilder.WriteString(") (")

	// Возвращаемые значения
	results := r.resultsWithoutError(method)
	if len(results) > 0 {
		for i, result := range results {
			if i > 0 {
				sigBuilder.WriteString(", ")
			}
			if result.Name != "" {
				sigBuilder.WriteString(result.Name)
				sigBuilder.WriteString(" ")
			}
			typeStr := r.goTypeStringFromVariable(result, contract.PkgPath)
			sigBuilder.WriteString(typeStr)
		}
		sigBuilder.WriteString(", ")
	}
	sigBuilder.WriteString("err error)")

	md.CodeBlocks(markdown.SyntaxHighlightGo, sigBuilder.String())
	md.LF()
}

// renderMethodParamsAndResults генерирует таблицы параметров и возвращаемых значений
func (r *ClientRenderer) renderMethodParamsAndResults(md *markdown.Markdown, method *core.Method, contract *core.Contract, typeUsages map[string]*typeUsage) {
	args := r.argsWithoutContext(method)
	results := r.resultsWithoutError(method)

	// Параметры
	if len(args) > 0 {
		md.PlainText(markdown.Bold("Параметры:"))
		md.LF()

		rows := make([][]string, 0)
		for _, arg := range args {
			typeStr := r.goTypeStringFromVariable(arg, contract.PkgPath)
			argTags := r.parseTagsFromDocs(strings.Join(arg.Docs, "\n"))
			argDesc := argTags[tagDesc]

			// Ссылка на тип (аналогично полям структуры)
			typeLink := r.getTypeLinkFromVariable(arg, contract.PkgPath)

			rows = append(rows, []string{
				markdown.Code(arg.Name),
				markdown.Code(typeStr),
				argDesc,
				typeLink,
			})
		}

		headers := []string{"Имя", "Тип", "Описание", "Ссылка на тип"}
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
			typeStr := r.goTypeStringFromVariable(result, contract.PkgPath)
			resultTags := r.parseTagsFromDocs(strings.Join(result.Docs, "\n"))
			resultDesc := resultTags[tagDesc]

			// Ссылка на тип (аналогично полям структуры)
			typeLink := r.getTypeLinkFromVariable(result, contract.PkgPath)

			resultName := result.Name
			if resultName == "" {
				resultName = "result"
			}

			rows = append(rows, []string{
				markdown.Code(resultName),
				markdown.Code(typeStr),
				resultDesc,
				typeLink,
			})
		}

		// Ошибка
		rows = append(rows, []string{
			markdown.Code("err"),
			markdown.Code("error"),
			"Ошибка выполнения запроса",
			"-",
		})

		headers := []string{"Имя", "Тип", "Описание", "Ссылка на тип"}
		tableSet := markdown.TableSet{
			Header: headers,
			Rows:   rows,
		}
		md.Table(tableSet)
		md.LF()
	}
}

// renderMethodErrors генерирует описание возможных ошибок метода
func (r *ClientRenderer) renderMethodErrors(md *markdown.Markdown, method *core.Method, contract *core.Contract) {
	// Используем информацию об ошибках из метода
	if len(method.Errors) == 0 {
		return
	}

	md.LF()
	md.PlainText(markdown.Bold("Возможные ошибки:"))
	md.LF()

	// Сортируем ошибки по HTTP коду для детерминированного порядка
	errors := make([]*core.ErrorInfo, len(method.Errors))
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

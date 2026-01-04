// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strconv"
	"strings"

	"tgp/plugins/client-ts/tsg"
	"tgp/shared"
)

// isHTTP проверяет, является ли метод HTTP методом.
func (r *ClientRenderer) isHTTP(method *shared.Method, contract *shared.Contract) bool {
	if method == nil || method.Annotations == nil {
		return false
	}
	// Проверяем, есть ли аннотация http-method
	return r.contains(method.Annotations, TagMethodHTTP)
}

// renderHTTPMethod генерирует HTTP метод клиента
func (r *ClientRenderer) renderHTTPMethod(grp *tsg.Group, method *shared.Method, contract *shared.Contract) {
	args := r.argsWithoutContext(method)
	results := r.resultsWithoutError(method)

	// Комментарий к методу (без аннотаций @tg)
	filteredDocs := r.filterDocsComments(method.Docs)
	if len(filteredDocs) > 0 {
		grp.Comment(strings.Join(filteredDocs, "\n"))
	} else {
		grp.Comment(fmt.Sprintf("Вызывает HTTP метод %s", method.Name))
	}

	// Добавляем информацию о возможных ошибках в JSDoc
	methodErrors := r.collectMethodErrors(method, contract)
	if len(methodErrors) > 0 {
		grp.Comment("@throws {Error} - Possible errors:")
		for _, errInfo := range methodErrors {
			typeName := fmt.Sprintf("%s%s", strings.Split(errInfo.pkgPath, "/")[len(strings.Split(errInfo.pkgPath, "/"))-1], errInfo.typeName)
			if errInfo.code != 0 {
				grp.Comment(fmt.Sprintf("  - %s (HTTP %d: %s)", typeName, errInfo.code, errInfo.codeText))
			} else {
				grp.Comment(fmt.Sprintf("  - %s", typeName))
			}
		}
	}

	// Определяем HTTP метод
	httpMethod := "POST"
	if r.contains(method.Annotations, TagMethodHTTP) {
		httpMethod = strings.ToUpper(annotationValue(method.Annotations, TagMethodHTTP, "POST"))
	}

	// Создаём параметры метода - отдельные параметры как в оригинальном интерфейсе
	methodParams := tsg.NewStatement()
	methodParams.Params(func(pg *tsg.Group) {
		if len(args) > 0 {
			// Генерируем отдельные параметры для каждого аргумента
			for _, arg := range args {
				typeStr := r.walkVariable(arg.Name, contract.PkgPath, arg, method.Annotations, true).typeLink()
				paramStmt := tsg.NewStatement()
				paramStmt.Id(arg.Name)
				if r.contains(method.Annotations, "nullable") {
					paramStmt.Optional()
				}
				paramStmt.Colon()
				paramStmt.Add(tsg.TypeFromString(typeStr))
				pg.Add(paramStmt)
			}
		}
	})

	// Тип возвращаемого значения
	returnType := r.resultToTypeStatement(method, results)

	// Получаем типы из exchange только если они нужны
	var requestTypeName string
	if len(args) > 0 {
		requestTypeName = r.requestTypeName(contract, method)
	}
	var responseTypeName string
	if len(results) > 0 {
		responseTypeName = r.responseTypeName(contract, method)
	}

	// Создаём async метод (публичный метод класса)
	methodStmt := tsg.NewStatement()
	methodStmt.Public()
	methodStmt.AsyncMethodWithParams(r.lcName(method.Name), methodParams, returnType, func(mg *tsg.Group) {
		// Собираем объект params из отдельных параметров с типизацией через exchange тип
		if len(args) > 0 {
			paramsObj := tsg.NewStatement()
			paramsObj.Const("params").Colon().Id(requestTypeName).Op("=")
			paramsObj.Values(func(vg *tsg.Group) {
				for _, arg := range args {
					vg.Add(tsg.NewStatement().Id(arg.Name).Colon().Id(arg.Name))
				}
			})
			mg.Add(paramsObj.Semicolon())
		}

		// Формируем URL
		urlStmt := tsg.NewStatement()
		urlStmt.Const("baseURL").Op("=").Id("this").Dot("baseClient").Dot("getEndpoint").Call().Semicolon()
		mg.Add(urlStmt)

		// Определяем путь
		path := r.httpPath(method, contract)
		urlStmt2 := tsg.NewStatement()
		urlStmt2.Var("url").Op("=").Id("baseURL")
		ternaryExpr := tsg.NewStatement()
		ternaryExpr.Op("(").Id("baseURL").Dot("endsWith").Call(tsg.NewStatement().Lit("/")).Op("?").Lit("").Op(":").Lit("/").Op(")")
		urlStmt2.Op("+").Add(ternaryExpr)
		urlStmt2.Op("+").Lit(strings.TrimPrefix(path, "/"))
		mg.Add(urlStmt2.Semicolon())

		// Формируем тело запроса
		bodyStmt := tsg.NewStatement()
		if len(args) > 0 {
			bodyObj := tsg.NewStatement()
			bodyObj.Values(func(bg *tsg.Group) {
				for _, arg := range args {
					bg.Add(tsg.NewStatement().Id(arg.Name).Colon().Id("params").Dot(arg.Name))
				}
			})
			bodyStmt.Const("body").Op("=").Id("JSON").Dot("stringify").Call(bodyObj)
		} else {
			bodyStmt.Const("body").Op("=").Lit("null")
		}
		mg.Add(bodyStmt.Semicolon())

		// Получаем заголовки из базового клиента (поддерживает статичные и динамические)
		headersVar := tsg.NewStatement().
			Const("clientHeaders").
			Colon().
			Id("Record").
			Generic("string", "string").
			Op("=").
			Await(tsg.NewStatement().Id("this").Dot("baseClient").Dot("getHeaders").Call()).
			Semicolon()
		mg.Add(headersVar)

		// Формируем заголовки
		headersStmt := tsg.NewStatement()
		headersStmt.Const("headers").Op("=").Id("new Headers").Call().Semicolon()
		mg.Add(headersStmt)
		mg.Add(tsg.NewStatement().Id("headers").Dot("set").Call(tsg.NewStatement().Lit("Content-Type"), tsg.NewStatement().Lit("application/json")).Semicolon())
		mg.Add(tsg.NewStatement().Id("headers").Dot("set").Call(tsg.NewStatement().Lit("Accept"), tsg.NewStatement().Lit("application/json")).Semicolon())
		
		// Добавляем заголовки из клиента
		mg.Add(tsg.NewStatement().
			ForOf("[key, value]", "Object.entries(clientHeaders)", func(fg *tsg.Group) {
				fg.Add(tsg.NewStatement().Id("headers").Dot("set").Call(tsg.NewStatement().Id("key"), tsg.NewStatement().Id("value")))
			}).
			Semicolon())

		// Выполняем запрос
		fetchOptions := tsg.NewStatement()
		fetchOptions.Values(func(fg *tsg.Group) {
			fg.Add(tsg.NewStatement().Id("method").Colon().Lit(httpMethod))
			fg.Add(tsg.NewStatement().Id("headers").Colon().Id("headers"))
			fg.Add(tsg.NewStatement().Id("body").Colon().Id("body"))
		})
		fetchStmt := tsg.NewStatement()
		fetchStmt.Const("response").Op("=").Await(tsg.NewStatement().Id("fetch").Call(tsg.NewStatement().Id("url"), fetchOptions))
		mg.Add(fetchStmt.Semicolon())

		// Проверяем статус код
		successCode := 200
		if r.contains(method.Annotations, TagHttpSuccess) {
			if code, err := strconv.Atoi(annotationValue(method.Annotations, TagHttpSuccess, "200")); err == nil {
				successCode = code
			}
		}

		mg.If(tsg.NewStatement().Id("response").Dot("status").Op("!=").Lit(successCode), func(ig *tsg.Group) {
			ig.Add(tsg.NewStatement().Const("errorBody").Op("=").Await(tsg.NewStatement().Id("response").Dot("text").Call()).Semicolon())
			if len(methodErrors) > 0 {
				unionTypeName := fmt.Sprintf("%sError", method.Name)
				ig.Try(
					func(tg *tsg.Group) {
						tg.Add(tsg.NewStatement().Const("errorData").Op("=").Id("JSON.parse").Call(tsg.NewStatement().Id("errorBody")).Semicolon())
						tg.Add(tsg.NewStatement().Const("error").Colon().Id(unionTypeName).Op("=").Id("errorData").Op("as").Id(unionTypeName).Semicolon())
						tg.Throw(tsg.NewStatement().Id("error"))
					},
					func(cg *tsg.Group) {
						cg.If(
							tsg.NewStatement().Id("e").Op("&&").Typeof(tsg.NewStatement().Id("e"), "object").Op("&&").In("message", tsg.NewStatement().Id("e")),
							func(ig *tsg.Group) {
								ig.Throw(tsg.NewStatement().Id("e"))
							},
						)
						cg.Throw(tsg.NewStatement().New("Error").Call(tsg.NewStatement().TemplateString(
							[]string{fmt.Sprintf("HTTP error: %d. ", successCode), ""},
							[]*tsg.Statement{tsg.NewStatement().Id("errorBody")},
						)))
					},
				)
			} else {
				ig.Throw(tsg.NewStatement().New("Error").Call(tsg.NewStatement().Lit(fmt.Sprintf("HTTP error: %d. ", successCode)).Op("+").Id("errorBody")))
			}
		})

		// Обрабатываем ответ с типизацией через exchange тип
		if len(results) == 0 {
			mg.Return()
		} else {
			// Типизируем responseData через exchange тип
			mg.Add(tsg.NewStatement().Const("responseData").Colon().Id(responseTypeName).Op("=").Await(tsg.NewStatement().Id("response").Dot("json").Call()).Op("as").Id(responseTypeName).Semicolon())
			if len(results) == 1 {
				mg.Return(tsg.NewStatement().Id("responseData"))
			} else {
				returnObj := tsg.NewStatement()
				returnObj.Values(func(rg *tsg.Group) {
					for _, ret := range results {
						rg.Add(tsg.NewStatement().Id(ret.Name).Colon().Id("responseData").Dot(ret.Name))
					}
				})
				mg.Return(returnObj)
			}
		}
	})
	grp.Add(methodStmt)
	grp.Line()
}

// httpPath возвращает путь для HTTP метода
func (r *ClientRenderer) httpPath(method *shared.Method, contract *shared.Contract) string {
	// Проверяем аннотацию http-path
	if r.contains(method.Annotations, TagHttpPath) {
		return annotationValue(method.Annotations, TagHttpPath, "")
	}
	// По умолчанию используем имя метода в lowerCamelCase
	return "/" + r.lcName(method.Name)
}


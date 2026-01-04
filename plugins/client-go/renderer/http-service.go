// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/shared"
)

// httpClientMethodFunc генерирует метод для HTTP вызова
func (r *ClientRenderer) httpClientMethodFunc(ctx context.Context, contract *shared.Contract, method *shared.Method, outDir string) Code {

	c := Comment(fmt.Sprintf("%s performs the %s operation.", method.Name, method.Name))
	c.Line()
	c.Func().Params(Id("cli").Op("*").Id("Client" + contract.Name)).
		Id(method.Name).
		Params(r.funcDefinitionParams(ctx, method.Args)).Params(r.funcDefinitionParams(ctx, method.Results)).
		BlockFunc(func(bg *Group) {
			bg.Line()
			if r.HasMetrics() && r.contains(contract.Annotations, TagMetrics) {
				bg.Defer().Func().Params(Id("_begin").Qual(PackageTime, "Time")).Block(
					If(Id("cli").Dot("Client").Dot("metrics").Op("==").Nil()).Block(
						Return(),
					),
					Var().Defs(
						Id("success").Op("=").True(),
						Id("errCode").Op("=").Id("internalError"),
					),
					If(Err().Op("!=").Nil()).Block(
						Id("success").Op("=").False(),
						List(Id("ec"), Id("ok")).Op(":=").Err().Assert(Id("withErrorCode")),
						If(Id("ok")).Block(
							Id("errCode").Op("=").Id("ec").Dot("Code").Call(),
						),
					),
					If(Id("success")).Block(
						Id("cli").Dot("Client").Dot("metrics").Dot("RequestCount").Dot("WithLabelValues").Call(
						Lit("client_"+r.contractNameToLowerCamel(contract)),
						Lit(r.methodNameToLowerCamel(method)),
							Lit("true"),
							Lit("0")).
							Dot("Add").Call(Lit(1)),
						Id("cli").Dot("Client").Dot("metrics").Dot("RequestCountAll").Dot("WithLabelValues").Call(
						Lit("client_"+r.contractNameToLowerCamel(contract)),
						Lit(r.methodNameToLowerCamel(method)),
							Lit("true"),
							Lit("0")).
							Dot("Add").Call(Lit(1)),
						Id("cli").Dot("Client").Dot("metrics").Dot("RequestLatency").Dot("WithLabelValues").Call(
						Lit("client_"+r.contractNameToLowerCamel(contract)),
						Lit(r.methodNameToLowerCamel(method)),
							Lit("true"),
							Lit("0")).
							Dot("Observe").Call(Qual(PackageTime, "Since").Call(Id("_begin")).Dot("Seconds").Call()),
					).Else().Block(
						Id("errCodeStr").Op(":=").Qual(PackageStrconv, "Itoa").Call(Id("errCode")),
						Id("cli").Dot("Client").Dot("metrics").Dot("RequestCount").Dot("WithLabelValues").Call(
						Lit("client_"+r.contractNameToLowerCamel(contract)),
						Lit(r.methodNameToLowerCamel(method)),
							Lit("false"),
							Id("errCodeStr")).
							Dot("Add").Call(Lit(1)),
						Id("cli").Dot("Client").Dot("metrics").Dot("RequestCountAll").Dot("WithLabelValues").Call(
						Lit("client_"+r.contractNameToLowerCamel(contract)),
						Lit(r.methodNameToLowerCamel(method)),
							Lit("false"),
							Id("errCodeStr")).
							Dot("Add").Call(Lit(1)),
						Id("cli").Dot("Client").Dot("metrics").Dot("RequestLatency").Dot("WithLabelValues").Call(
						Lit("client_"+r.contractNameToLowerCamel(contract)),
						Lit(r.methodNameToLowerCamel(method)),
							Lit("false"),
							Id("errCodeStr")).
							Dot("Observe").Call(Qual(PackageTime, "Since").Call(Id("_begin")).Dot("Seconds").Call()),
					),
				).Call(Qual(PackageTime, "Now").Call())
			}
			var httpMethod string
			if r.contains(method.Annotations, TagMethodHTTP) {
				if val, ok := method.Annotations[TagMethodHTTP]; ok {
					httpMethod = val
				}
			} else {
				httpMethod = "POST"
			}
			var successStatusCode int
			if r.contains(method.Annotations, TagHttpSuccess) {
				successCodeStr := method.Annotations[TagHttpSuccess]
				code, err := strconv.Atoi(successCodeStr)
				if err != nil {
					successStatusCode = http.StatusOK
				} else {
					successStatusCode = code
				}
			} else {
				successStatusCode = http.StatusOK
			}
			methodPath := strings.ToLower(method.Name)
			pathParams := r.argPathMap(method)
			if r.contains(method.Annotations, TagHttpPath) {
				if val, ok := method.Annotations[TagHttpPath]; ok {
					methodPath = val
				}
			}
			svcPrefix := contract.Name
			if r.contains(contract.Annotations, TagHttpPrefix) {
				if val, ok := contract.Annotations[TagHttpPrefix]; ok {
					svcPrefix = val
				}
			}
			argsMappings := r.argParamMap(method)
			cookieMappings := r.varCookieMap(method)
			headerMappings := r.varHeaderMap(method)

			// Формируем URL
			fullURLPath := path.Join(svcPrefix, methodPath)
			bg.Var().Id("urlStr").String()
			if len(pathParams) > 0 {
				bg.Id("urlPath").Op(":=").Lit(fullURLPath)
				for _, paramName := range pathParams {
					paramVar := r.argByName(method, paramName)
					escapedValue := Qual("net/url", "PathEscape").Call(r.varToString(ctx, paramVar))
					bg.Id("urlPath").Op("=").Qual(PackageStrings, "Replace").Call(Id("urlPath"), Lit(":"+paramName), escapedValue, Lit(-1))
				}
				bg.Id("urlStr").Op("=").Id("cli").Dot("endpoint").Op("+").Lit("/").Op("+").Id("urlPath")
			} else {
				bg.Id("urlStr").Op("=").Id("cli").Dot("endpoint").Op("+").Lit("/").Op("+").Lit(fullURLPath)
			}

			// Добавляем query параметры
			hasQueryParams := false
			for paramName := range argsMappings {
				if r.argByName(method, paramName) != nil {
					hasQueryParams = true
					break
				}
			}
			if hasQueryParams {
				bg.Id("queryValues").Op(":=").Qual("net/url", "Values").Values(Dict{})
				for paramName, argName := range argsMappings {
					paramVar := r.argByName(method, paramName)
					if paramVar == nil {
						continue
					}
					if paramVar.NumberOfPointers > 0 {
						bg.If(Id(ToLowerCamel(paramName)).Op("!=").Nil()).Block(
							Id("queryValues").Dot("Set").Call(Lit(argName), r.varToString(ctx, paramVar)),
						)
					} else {
						bg.Id("queryValues").Dot("Set").Call(Lit(argName), r.varToString(ctx, paramVar))
					}
				}
				bg.Id("urlStr").Op("+=").Lit("?").Op("+").Id("queryValues").Dot("Encode").Call()
			}

			// Проверяем, есть ли тело запроса (параметры, которые не в path, query, header, cookie)
			hasBody := false
			argsWithoutCtx := r.argsWithoutContext(method)
			for _, arg := range argsWithoutCtx {
				if _, exists := argsMappings[arg.Name]; exists {
					continue
				}
				if _, exists := cookieMappings[arg.Name]; exists {
					continue
				}
				if _, exists := headerMappings[arg.Name]; exists {
					continue
				}
				if _, exists := pathParams[arg.Name]; exists {
					continue
				}
				hasBody = true
				break
			}
			if hasBody {
				bg.Var().Id("reqBody").Qual(PackageBytes, "Buffer")
				bg.Id("_request").Op(":=").Id(r.requestStructName(contract, method)).Values(DictFunc(func(dict Dict) {
					for idx, arg := range argsWithoutCtx {
						if _, exists := argsMappings[arg.Name]; exists {
							continue
						}
						if _, exists := cookieMappings[arg.Name]; exists {
							continue
						}
						if _, exists := headerMappings[arg.Name]; exists {
							continue
						}
						if _, exists := pathParams[arg.Name]; exists {
							continue
						}
						dict[Id(ToCamel(arg.Name))] = Id(ToLowerCamel(argsWithoutCtx[idx].Name))
					}
				}))
				jsonPkg := PackageStdJSON
				if val, ok := contract.Annotations[tagPackageJSON]; ok {
					jsonPkg = val
				}
				bg.If(Err().Op("=").Qual(jsonPkg, "NewEncoder").Call(Op("&").Id("reqBody")).Dot("Encode").Call(Id("_request")).Op(";").Err().Op("!=").Nil()).Block(
					Return(),
				)
			}

			// Создаём HTTP запрос
			bg.Var().Id("httpReq").Op("*").Qual(PackageHttp, "Request")
			if hasBody {
				bg.List(Id("httpReq"), Err()).Op("=").Qual(PackageHttp, "NewRequestWithContext").Call(Id(_ctx_), Lit(httpMethod), Id("urlStr"), Qual(PackageBytes, "NewReader").Call(Id("reqBody").Dot("Bytes").Call()))
			} else {
				bg.List(Id("httpReq"), Err()).Op("=").Qual(PackageHttp, "NewRequestWithContext").Call(Id(_ctx_), Lit(httpMethod), Id("urlStr"), Nil())
			}
			bg.If(Err().Op("!=").Nil()).Block(
				Return(),
			)

			// Устанавливаем заголовки
			bg.Id("httpReq").Dot("Header").Dot("Set").Call(Lit("Accept"), Lit("application/json"))
			if hasBody {
				bg.Id("httpReq").Dot("Header").Dot("Set").Call(Lit("Content-Type"), Lit("application/json"))
			}
			for paramName, headerName := range headerMappings {
				if paramVar := r.argByName(method, paramName); paramVar != nil {
					bg.Id("httpReq").Dot("Header").Dot("Set").Call(Lit(headerName), r.varToString(ctx, paramVar))
				}
			}
			for paramName, cookieName := range cookieMappings {
				if paramVar := r.argByName(method, paramName); paramVar != nil {
					bg.Id("httpReq").Dot("AddCookie").Call(Op("&").Qual(PackageHttp, "Cookie").Values(Dict{
						Id("Name"):  Lit(cookieName),
						Id("Value"): r.varToString(ctx, paramVar),
					}))
				}
			}

			// Добавляем заголовки из контекста
			bg.For(List(Id("_"), Id("header")).Op(":=").Range().Id("cli").Dot("Client").Dot("headersFromCtx")).Block(
				If(Id("value").Op(":=").Id(_ctx_).Dot("Value").Call(Id("header")).Op(";").Id("value").Op("!=").Nil()).Block(
					Var().Id("k").String(),
					Var().Id("v").String(),
					// Эффективная конвертация ключа
					If(List(Id("h"), Id("ok")).Op(":=").Id("header").Assert(String()).Op(";").Id("ok")).Block(
						Id("k").Op("=").Id("h"),
					).Else().If(List(Id("h"), Id("ok")).Op(":=").Id("header").Assert(Qual(PackageFmt, "Stringer")).Op(";").Id("ok")).Block(
						Id("k").Op("=").Id("h").Dot("String").Call(),
					).Else().Block(
						Id("k").Op("=").Qual(PackageFmt, "Sprint").Call(Id("header")),
					),
					// Эффективная конвертация значения
					If(List(Id("val"), Id("ok")).Op(":=").Id("value").Assert(String()).Op(";").Id("ok")).Block(
						Id("v").Op("=").Id("val"),
					).Else().If(List(Id("val"), Id("ok")).Op(":=").Id("value").Assert(Qual(PackageFmt, "Stringer")).Op(";").Id("ok")).Block(
						Id("v").Op("=").Id("val").Dot("String").Call(),
					).Else().Block(
						Id("v").Op("=").Qual(PackageFmt, "Sprint").Call(Id("value")),
					),
					If(Id("k").Op("!=").Lit("").Op("&&").Id("v").Op("!=").Lit("")).Block(
						Id("httpReq").Dot("Header").Dot("Set").Call(Id("k"), Id("v")),
					),
				),
			)

			// Вызываем BeforeRequest hook, если установлен
			bg.If(Id("cli").Dot("Client").Dot("beforeRequest").Op("!=").Nil()).Block(
				Id(_ctx_).Op("=").Id("cli").Dot("Client").Dot("beforeRequest").Call(Id(_ctx_), Id("httpReq")),
			)

			// Логируем запрос, если включено
			bg.If(Id("cli").Dot("Client").Dot("logRequests")).Block(
				If(List(Id("cmd"), Id("cmdErr")).Op(":=").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ToCurl").Call(Id("httpReq")).Op(";").Id("cmdErr").Op("==").Nil()).Block(
					Qual(PackageSlog, "DebugContext").Call(Id(_ctx_), Lit("HTTP request"), Qual(PackageSlog, "String").Call(Lit("method"), Id("httpReq").Dot("Method")), Qual(PackageSlog, "String").Call(Lit("curl"), Id("cmd").Dot("String").Call())),
				),
			)

			// Выполняем запрос
			bg.Var().Id("httpResp").Op("*").Qual(PackageHttp, "Response")
			bg.List(Id("httpResp"), Err()).Op("=").Id("cli").Dot("httpClient").Dot("Do").Call(Id("httpReq"))

			// Логируем ошибку, если включено
			bg.Defer().Func().Params().Block(
				If(Err().Op("!=").Nil().Op("&&").Id("cli").Dot("Client").Dot("logOnError").Op("&&").Id("httpReq").Op("!=").Nil()).Block(
					If(List(Id("cmd"), Id("cmdErr")).Op(":=").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ToCurl").Call(Id("httpReq")).Op(";").Id("cmdErr").Op("==").Nil()).Block(
						Qual(PackageSlog, "ErrorContext").Call(Id(_ctx_), Lit("HTTP request failed"), Qual(PackageSlog, "String").Call(Lit("method"), Id("httpReq").Dot("Method")), Qual(PackageSlog, "String").Call(Lit("curl"), Id("cmd").Dot("String").Call()), Qual(PackageSlog, "Any").Call(Lit("error"), Err())),
					),
				),
			).Call()
			bg.If(Err().Op("!=").Nil()).Block(
				Return(),
			)
			bg.Defer().Id("httpResp").Dot("Body").Dot("Close").Call()

			// Вызываем AfterRequest hook, если установлен
			bg.If(Id("cli").Dot("Client").Dot("afterRequest").Op("!=").Nil()).Block(
				If(Err().Op("=").Id("cli").Dot("Client").Dot("afterRequest").Call(Id(_ctx_), Id("httpResp")).Op(";").Err().Op("!=").Nil()).Block(
					Return(),
				),
			)

			// Проверяем статус код
			bg.If(Id("httpResp").Dot("StatusCode").Op("!=").Lit(successStatusCode)).Block(
				Var().Id("respBodyBytes").Index().Byte(),
				List(Id("respBodyBytes"), Err()).Op("=").Qual(PackageIO, "ReadAll").Call(Id("httpResp").Dot("Body")),
				If(Err().Op("!=").Nil()).Block(
					Err().Op("=").Qual(PackageFmt, "Errorf").Call(
						Lit("HTTP error: %d. URL: %s, Method: %s"),
						Id("httpResp").Dot("StatusCode"),
						Id("httpReq").Dot("URL").Dot("String").Call(),
						Id("httpReq").Dot("Method"),
					),
				).Else().Block(
					Err().Op("=").Qual(PackageFmt, "Errorf").Call(
						Lit("HTTP error: %d. URL: %s, Method: %s, Body: %s"),
						Id("httpResp").Dot("StatusCode"),
						Id("httpReq").Dot("URL").Dot("String").Call(),
						Id("httpReq").Dot("Method"),
						String().Call(Id("respBodyBytes")),
					),
				),
				Return(),
			)

			// Потоковое чтение JSON ответа
			resultsWithoutErr := r.resultsWithoutError(method)
			fieldsResult := r.fieldsResult(method)
			if len(resultsWithoutErr) == 1 && r.contains(method.Annotations, TagHttpEnableInlineSingle) {
				bg.Var().Id("_response").Id(r.responseStructName(contract, method))
				jsonPkg := PackageStdJSON
				if val, ok := contract.Annotations[tagPackageJSON]; ok {
					jsonPkg = val
				}
				bg.Var().Id("decoder").Op("=").Qual(jsonPkg, "NewDecoder").Call(Id("httpResp").Dot("Body"))
				bg.If(Op("!").Id("cli").Dot("Client").Dot("allowUnknownFields")).Block(
					Id("decoder").Dot("DisallowUnknownFields").Call(),
				)
				bg.If(Err().Op("=").Id("decoder").Dot("Decode").Call(Op("&").Id("_response").Dot(ToCamel(resultsWithoutErr[0].Name))).Op(";").Err().Op("!=").Nil()).Block(
					Return(),
				)
				// fieldsResult и resultsWithoutErr имеют одинаковый порядок и количество элементов
				for i, ret := range resultsWithoutErr {
					if i >= len(fieldsResult) {
						// Если поле не найдено, используем значение по умолчанию
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Add(Id("_response").Dot(ToCamel(ret.Name)))
						continue
					}
					field := fieldsResult[i]
					fieldValue := Id("_response").Dot(ToCamel(ret.Name))
					// Если поле в структуре - указатель, а возвращаемое значение - не указатель, разыменовываем
					switch {
					case field.numberOfPointers > 0 && ret.NumberOfPointers == 0:
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Op("*").Add(fieldValue)
					case field.numberOfPointers == 0 && ret.NumberOfPointers > 0:
						// Если поле в структуре - не указатель, а возвращаемое значение - указатель, берем адрес
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Op("&").Add(fieldValue)
					default:
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Add(fieldValue)
					}
				}
			} else {
				bg.Var().Id("_response").Id(r.responseStructName(contract, method))
				jsonPkg := PackageStdJSON
				if val, ok := contract.Annotations[tagPackageJSON]; ok {
					jsonPkg = val
				}
				bg.Var().Id("decoder").Op("=").Qual(jsonPkg, "NewDecoder").Call(Id("httpResp").Dot("Body"))
				bg.If(Op("!").Id("cli").Dot("Client").Dot("allowUnknownFields")).Block(
					Id("decoder").Dot("DisallowUnknownFields").Call(),
				)
				bg.If(Err().Op("=").Id("decoder").Dot("Decode").Call(Op("&").Id("_response")).Op(";").Err().Op("!=").Nil()).Block(
					Return(),
				)
				// fieldsResult и resultsWithoutErr имеют одинаковый порядок и количество элементов
				for i, ret := range resultsWithoutErr {
					if i >= len(fieldsResult) {
						// Если поле не найдено, используем значение по умолчанию
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Add(Id("_response").Dot(ToCamel(ret.Name)))
						continue
					}
					field := fieldsResult[i]
					fieldValue := Id("_response").Dot(ToCamel(ret.Name))
					// Если поле в структуре - указатель, а возвращаемое значение - не указатель, разыменовываем
					switch {
					case field.numberOfPointers > 0 && ret.NumberOfPointers == 0:
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Op("*").Add(fieldValue)
					case field.numberOfPointers == 0 && ret.NumberOfPointers > 0:
						// Если поле в структуре - не указатель, а возвращаемое значение - указатель, берем адрес
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Op("&").Add(fieldValue)
					default:
						bg.Id(ToLowerCamel(ret.Name)).Op("=").Add(fieldValue)
					}
				}
			}
			bg.Return()
		})
	return c
}

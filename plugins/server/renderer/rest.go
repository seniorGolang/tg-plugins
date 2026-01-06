// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/internal/parser"
	"tgp/plugins/server/renderer/types"
)

// RenderREST генерирует REST обработчики.
func (r *contractRenderer) RenderREST() error {

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackageZeroLog, "zerolog")
	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))
	srcFile.ImportName(jsonPkg, "json")
	srcFile.ImportName(PackageReflect, "reflect")
	srcFile.ImportName(PackageFmt, "fmt")

	typeGen := types.NewGenerator(r.project, &srcFile)

	for _, method := range r.contract.Methods {
		if !r.methodIsHTTP(method) {
			continue
		}
		srcFile.Add(r.httpMethodFunc(typeGen, method))
		srcFile.Add(r.httpServeMethodFunc(&srcFile, typeGen, method, jsonPkg))
	}

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-rest.go"))
}

// httpMethodFunc генерирует функцию обработки HTTP метода.
func (r *contractRenderer) httpMethodFunc(typeGen *types.Generator, method *parser.Method) Code {

	return Func().Params(Id("http").Op("*").Id("http"+r.contract.Name)).
		Id(toLowerCamel(method.Name)).
		Params(Id(VarNameCtx).Qual(PackageContext, "Context"), Id("request").Id(requestStructName(r.contract.Name, method.Name))).
		Params(Id("response").Id(responseStructName(r.contract.Name, method.Name)), Err().Error()).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.ListFunc(func(lg *Group) {
				for _, ret := range r.ResultFieldsWithoutError(method) {
					lg.Id("response").Dot(toCamel(ret.Name))
				}
				lg.Err()
			}).Op("=").Id("http").Dot("svc").Dot(method.Name).CallFunc(func(cg *Group) {
				cg.Id(VarNameCtx)
				for _, arg := range r.ArgsFieldsWithoutContext(method) {
					argCode := Id("request").Dot(toCamel(arg.Name))
					if arg.IsEllipsis {
						argCode.Op("...")
					}
					cg.Add(argCode)
				}
			})
			bg.If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.If(Id("http").Dot("errorHandler").Op("!=").Nil()).Block(
					Err().Op("=").Id("http").Dot("errorHandler").Call(Err()),
				)
			})
			bg.Return()
		})
}

// httpServeMethodFunc генерирует функцию обработки HTTP запроса.
func (r *contractRenderer) httpServeMethodFunc(srcFile *GoFile, typeGen *types.Generator, method *parser.Method, jsonPkg string) Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("serve" + method.Name).
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).
		Params(Err().Error()).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Var().Id("request").Id(requestStructName(r.contract.Name, method.Name))
			if successCodeStr := method.Annotations.Value(TagHttpSuccess, ""); successCodeStr != "" {
				if successCode, err := strconv.Atoi(successCodeStr); err == nil && successCode != 0 {
					bg.Id(VarNameFtx).Dot("Response").Call().Dot("SetStatusCode").Call(Lit(successCode))
				}
			}
			if len(r.arguments(method)) != 0 {
				bg.If(Err().Op("=").Id(VarNameFtx).Dot("BodyParser").Call(Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
					ig.Id(VarNameFtx).Dot("Response").Call().Dot("SetStatusCode").Call(Qual(PackageFiber, "StatusBadRequest"))
					ig.List(Id("_"), Err()).Op("=").Id(VarNameFtx).Dot("WriteString").Call(Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call())
					ig.Return()
				})
			}
			bg.Add(r.urlArgs(srcFile, typeGen, method, func(arg, header string) *Statement {
				return Line().If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
					ig.Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusBadRequest"))
					ig.Return().Id("sendResponse").Call(Id(VarNameFtx), Lit("path arguments could not be decoded: ").Op("+").Err().Dot("Error").Call())
				})
			}))
			bg.Add(r.urlParams(srcFile, typeGen, method, func(arg, header string) *Statement {
				return Line().If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
					ig.Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusBadRequest"))
					ig.Return().Id("sendResponse").Call(Id(VarNameFtx), Lit("url arguments could not be decoded: ").Op("+").Err().Dot("Error").Call())
				})
			}))
			bg.Add(r.httpArgHeaders(srcFile, typeGen, method, func(arg, header string) *Statement {
				return Line().If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
					ig.Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusBadRequest"))
					ig.Return().Id("sendResponse").Call(Id(VarNameFtx), Lit("http header could not be decoded: ").Op("+").Err().Dot("Error").Call())
				})
			}))
			bg.Add(r.httpCookies(srcFile, typeGen, method, func(arg, header string) *Statement {
				return Line().If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
					ig.Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusBadRequest"))
					ig.Return().Id("sendResponse").Call(Id(VarNameFtx), Lit("http header could not be decoded: ").Op("+").Err().Dot("Error").Call())
				})
			}))
			if responseMethod := method.Annotations.Value(TagHttpResponse, ""); responseMethod != "" {
				// Для http-response передаем ftx, base (интерфейс сервиса) и параметры запроса напрямую в handler
				args := argsWithoutContext(method)
				callArgs := []Code{Id(VarNameFtx), Id("http").Dot("base")}
				for _, arg := range args {
					callArgs = append(callArgs, Id("request").Dot(toCamel(arg.Name)))
				}
				// Используем toIDWithImport для добавления импорта и вызова обработчика
				bg.Return().Add(toIDWithImport(responseMethod, srcFile).Call(callArgs...))
			} else {
				bg.Var().Id("response").Id(responseStructName(r.contract.Name, method.Name))
				bg.If().List(Id("response"), Err()).Op("=").Id("http").Dot(toLowerCamel(method.Name)).Call(Id(VarNameFtx).Dot("UserContext").Call(), Id("request")).Op(";").Err().Op("==").Nil().BlockFunc(func(bf *Group) {
					var ex Statement
					if len(r.retCookieMap(method)) > 0 {
						for retName := range r.retCookieMap(method) {
							if ret := r.resultByName(method, retName); ret != nil {
								ex.If(List(Id("rCookie"), Id("ok")).Op(":=").
									Qual(PackageReflect, "ValueOf").Call(Id("response").Dot(toCamel(retName))).Dot("Interface").Call().
									Op(".").Call(Id("cookieType"))).Op(";").Id("ok").Block(
									Id("cookie").Op(":=").Id("rCookie").Dot("Cookie").Call(),
									Id(VarNameFtx).Dot("Cookie").Call(Op("&").Id("cookie")),
								)
							}
						}
					}
					ex.Add(r.httpRetHeaders(method))
					bf.Var().Id("iResponse").Interface().Op("=").Id("response")
					bf.If(List(Id("redirect"), Id("ok")).Op(":=").Id("iResponse").Op(".").Call(Id("withRedirect")).Op(";").Id("ok")).Block(
						Return().Id(VarNameFtx).Dot("Redirect").Call(Id("redirect").Dot("RedirectTo").Call()),
					)
					if len(ex) > 0 {
						bf.Add(&ex)
					}
					if len(resultsWithoutError(method)) == 1 && method.Annotations.Contains(TagHttpEnableInlineSingle) {
						bf.Return().Id("sendResponse").Call(Id(VarNameFtx), Id("response").Dot(toCamel(resultsWithoutError(method)[0].Name)))
					} else {
						bf.Return().Id("sendResponse").Call(Id(VarNameFtx), Id("response"))
					}
				})
				bg.If(List(Id("errCoder"), Id("ok")).Op(":=").Err().Op(".").Call(Id("withErrorCode")).Op(";").Id("ok")).Block(
					Id(VarNameFtx).Dot("Status").Call(Id("errCoder").Dot("Code").Call()),
				).Else().Block(
					Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusInternalServerError")),
				)
				bg.Return().Id("sendResponse").Call(Id(VarNameFtx), Err())
			}
		})
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderHTTP генерирует HTTP обработчики для контракта.
func (r *contractRenderer) RenderHTTP() error {

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackageCors, "cors")
	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackageZeroLog, "zerolog")
	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))

	r.renderHTTPTypes(&srcFile)
	r.renderHTTPNewFunc(&srcFile)
	r.renderHTTPServiceFunc(&srcFile)
	r.renderHTTPWithFuncs(&srcFile)
	r.renderHTTPSetRoutes(&srcFile)

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-http.go"))
}

// renderHTTPTypes генерирует типы для HTTP обработчика.
func (r *contractRenderer) renderHTTPTypes(srcFile *GoFile) {

	fields := []Code{
		Id("errorHandler").Id("ErrorHandler"),
		Id("maxBatchSize").Int(),
		Id("maxParallelBatch").Int(),
		Id("svc").Op("*").Id("server" + r.contract.Name),
		Id("base").Qual(r.contract.PkgPath, r.contract.Name),
	}
	if r.contract.Annotations.Contains(TagServerJsonRPC) {
		fields = append(fields, Id("srv").Op("*").Id("Server"))
	}
	srcFile.Type().Id("http" + r.contract.Name).Struct(fields...)
}

// renderHTTPNewFunc генерирует функцию создания HTTP обработчика.
func (r *contractRenderer) renderHTTPNewFunc(srcFile *GoFile) {

	srcFile.Line().Func().Id("new"+r.contract.Name).
		Params(Id("svc"+r.contract.Name).Qual(r.contract.PkgPath, r.contract.Name)).
		Params(Id("srv").Op("*").Id("http"+r.contract.Name)).
		Block(
			Line().Id("srv").Op("=").Op("&").Id("http"+r.contract.Name).Values(Dict{
				Id("base"): Id("svc" + r.contract.Name),
				Id("svc"):  Id("newServer" + r.contract.Name).Call(Id("svc" + r.contract.Name)),
			}),
			Return(),
		)
}

// renderHTTPServiceFunc генерирует функцию получения сервиса.
func (r *contractRenderer) renderHTTPServiceFunc(srcFile *GoFile) {

	srcFile.Line().Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("Service").
		Params().
		Params(Op("*").Id("server" + r.contract.Name)).
		Block(
			Return(Id("http").Dot("svc")),
		)
}

// renderHTTPWithFuncs генерирует функции With* для HTTP обработчика.
func (r *contractRenderer) renderHTTPWithFuncs(srcFile *GoFile) {

	srcFile.Line().Add(r.httpWithLogFunc())
	if r.contract.Annotations.Contains(TagTrace) {
		srcFile.Line().Add(r.httpWithTraceFunc())
	}
	if r.contract.Annotations.Contains(TagMetrics) {
		srcFile.Line().Add(r.httpWithMetricsFunc())
	}
	srcFile.Line().Add(r.httpWithErrorHandler())
	if r.contract.Annotations.Contains(TagServerHTTP) {
		srcFile.Line().Add(r.httpWithRedirectFunc())
	}
}

// renderHTTPSetRoutes генерирует функцию установки маршрутов.
func (r *contractRenderer) renderHTTPSetRoutes(srcFile *GoFile) {

	srcFile.Line().Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("SetRoutes").
		Params(Id("route").Op("*").Qual(PackageFiber, "App")).
		BlockFunc(func(bg *Group) {
			if r.contract.Annotations.Contains(TagServerJsonRPC) {
				bg.Id("route").Dot("Post").Call(Lit(r.batchPath()), Id("http").Dot("serveBatch"))
				for _, method := range r.contract.Methods {
					if !r.methodIsJsonRPC(method) {
						continue
					}
					bg.Id("route").Dot("Post").Call(Lit(r.methodJsonRPCPath(method)), Id("http").Dot("serve"+method.Name))
				}
			}
			if r.contract.Annotations.Contains(TagServerHTTP) {
				for _, method := range r.contract.Methods {
					if !r.methodIsHTTP(method) {
						continue
					}
					if method.Annotations.Contains(TagHandler) {
						handlerQual := r.methodHandlerQual(srcFile, method)
						bg.Id("route").Dot(toCamel(r.methodHTTPMethod(method))).
							Call(Lit(r.methodHTTPPath(method)), Func().Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).Params(Err().Error()).Block(
								Return().Add(handlerQual).Call(Id(VarNameFtx), Id("http").Dot("base")),
							))
						continue
					}
					bg.Id("route").Dot(toCamel(r.methodHTTPMethod(method))).
						Call(Lit(r.methodHTTPPath(method)), Id("http").Dot("serve"+method.Name))
				}
			}
		})
}

// httpWithErrorHandler генерирует функцию WithErrorHandler.
func (r *contractRenderer) httpWithErrorHandler() Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("WithErrorHandler").
		Params(Id("handler").Id("ErrorHandler")).
		Params(Op("*").Id("http" + r.contract.Name)).
		BlockFunc(func(bg *Group) {
			bg.Id("http").Dot("errorHandler").Op("=").Id("handler")
			bg.Return(Id("http"))
		})
}

// httpWithLogFunc генерирует функцию WithLog.
func (r *contractRenderer) httpWithLogFunc() Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("WithLog").
		Params().
		Params(Op("*").Id("http" + r.contract.Name)).
		BlockFunc(func(bg *Group) {
			bg.Id("http").Dot("svc").Dot("WithLog").Call()
			bg.Return(Id("http"))
		})
}

// httpWithTraceFunc генерирует функцию WithTrace.
func (r *contractRenderer) httpWithTraceFunc() Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("WithTrace").
		Params().
		Params(Op("*").Id("http" + r.contract.Name)).
		BlockFunc(func(bg *Group) {
			bg.Id("http").Dot("svc").Dot("WithTrace").Call()
			bg.Return(Id("http"))
		})
}

// httpWithMetricsFunc генерирует функцию WithMetrics.
func (r *contractRenderer) httpWithMetricsFunc() Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("WithMetrics").
		Params(Id("metrics").Op("*").Id("Metrics")).
		Params(Op("*").Id("http" + r.contract.Name)).
		BlockFunc(func(bg *Group) {
			bg.Id("http").Dot("svc").Dot("WithMetrics").Call(Id("metrics"))
			bg.Return(Id("http"))
		})
}

// httpWithRedirectFunc генерирует функцию WithRedirect.
func (r *contractRenderer) httpWithRedirectFunc() Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("WithRedirect").
		Params().
		Params(Op("*").Id("http" + r.contract.Name)).
		BlockFunc(func(bg *Group) {
			bg.Return(Id("http"))
		})
}

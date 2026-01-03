// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// fiberFunc генерирует функцию Fiber.
func (r *transportRenderer) fiberFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("Fiber").
		Params().
		Params(Op("*").Qual(PackageFiber, "App")).
		Block(
			Return(Id("srv").Dot("srvHTTP")),
		)
}

// withLogFunc генерирует функцию WithLog.
func (r *transportRenderer) withLogFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("WithLog").
		Params().
		Params(Op("*").Id("Server")).
		BlockFunc(func(bg *Group) {
			if r.hasHTTPService() {
				bg.If(Id("srv").Dot("httpHTTPService").Op("!=").Nil()).Block(
					Id("srv").Dot("httpHTTPService").Op("=").Id("srv").Dot("HTTPService").Call().Dot("WithLog").Call(),
				)
			}
			// Применяем логирование для всех контрактов с jsonRPC
			for _, contract := range r.project.Contracts {
				if contract.Annotations.Contains(TagServerJsonRPC) {
					bg.If(Id("srv").Dot("http" + contract.Name).Op("!=").Nil()).Block(
						Id("srv").Dot("http" + contract.Name).Op("=").Id("srv").Dot("http" + contract.Name).Dot("WithLog").Call(),
					)
				}
			}
			bg.Return(Id("srv"))
		})
}

// withTraceFunc генерирует функцию WithTrace.
func (r *transportRenderer) withTraceFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("WithTrace").
		Params(Id(VarNameCtx).Qual(PackageContext, "Context"), Id("appName").String(), Id("endpoint").String(), Id("attributes").Op("...").Qual(PackageAttributeOTEL, "KeyValue")).
		Params(Op("*").Id("Server")).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Qual(fmt.Sprintf("%s/tracer", r.pkgPath(r.outDir)), "Init").Call(Id(VarNameCtx), Id("appName"), Id("endpoint"), Id("attributes").Op("..."))
			if r.hasHTTPService() {
				bg.If(Id("srv").Dot("httpHTTPService").Op("!=").Nil()).Block(
					Id("srv").Dot("httpHTTPService").Op("=").Id("srv").Dot("HTTPService").Call().Dot("WithTrace").Call(),
				)
			}
			// Применяем трейсинг для всех контрактов с jsonRPC
			for _, contract := range r.project.Contracts {
				if contract.Annotations.Contains(TagServerJsonRPC) {
					bg.If(Id("srv").Dot("http" + contract.Name).Op("!=").Nil()).Block(
						Id("srv").Dot("http" + contract.Name).Op("=").Id("srv").Dot("http" + contract.Name).Dot("WithTrace").Call(),
					)
				}
			}
			bg.Return(Id("srv"))
		})
}

// withMetricsFunc генерирует функцию WithMetrics.
func (r *transportRenderer) withMetricsFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("WithMetrics").
		Params().
		Params(Op("*").Id("Server")).
		BlockFunc(func(bg *Group) {
			bg.If(Id("srv").Dot("metrics").Op("==").Nil()).Block(
				Id("srv").Dot("metrics").Op("=").Id("NewMetrics").Call(),
			)
			if r.hasHTTPService() {
				bg.If(Id("srv").Dot("httpHTTPService").Op("!=").Nil()).Block(
					Id("srv").Dot("httpHTTPService").Op("=").Id("srv").Dot("HTTPService").Call().Dot("WithMetrics").Call(Id("srv").Dot("metrics")),
				)
			}
			// Применяем метрики для всех контрактов с jsonRPC
			for _, contract := range r.project.Contracts {
				if contract.Annotations.Contains(TagServerJsonRPC) {
					bg.If(Id("srv").Dot("http" + contract.Name).Op("!=").Nil()).Block(
						Id("srv").Dot("http" + contract.Name).Op("=").Id("srv").Dot("http" + contract.Name).Dot("WithMetrics").Call(Id("srv").Dot("metrics")),
					)
				}
			}
			bg.Return(Id("srv"))
		})
}

// serverNewFunc генерирует функцию New.
func (r *transportRenderer) serverNewFunc() Code {

	return Func().Id("New").
		Params(Id("log").Op("*").Qual(PackageSlog, "Logger"), Id("options").Op("...").Id("Option")).
		Params(Id("srv").Op("*").Id("Server")).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Id("srv").Op("=").Op("&").Id("Server").Values(DictFunc(func(dict Dict) {
				dict[Id("log")] = Id("log")
				if r.hasJsonRPC() {
					dict[Id("maxBatchSize")] = Id("defaultMaxBatchSize")
					dict[Id("maxParallelBatch")] = Id("defaultMaxParallelBatch")
					dict[Id("methodTimeout")] = Lit(30).Op("*").Qual(PackageTime, "Second")
				}
				dict[Id("headerHandlers")] = Make(Map(String()).Id("HeaderHandler"))
				dict[Id("config")] = Qual(PackageFiber, "Config").Values(Dict{
					Id("DisableStartupMessage"): True(),
					Id("BodyLimit"):             Id("defaultBodyLimit"),
					Id("ReadBufferSize"):        Id("defaultReadBufferSize"),
					Id("WriteBufferSize"):       Id("defaultWriteBufferSize"),
					Id("ReadTimeout"):           Id("defaultReadTimeout"),
					Id("WriteTimeout"):          Id("defaultWriteTimeout"),
					Id("IdleTimeout"):           Id("defaultIdleTimeout"),
					Id("Concurrency"):           Id("defaultConcurrency"),
				})
			},
			))
			bg.Line()
			bg.Var().Id("configOptions").Index().Id("Option")
			bg.Var().Id("serviceOptions").Index().Id("Option")
			bg.Line()
			bg.For(List(Id("_"), Id("option")).Op(":=").Range().Id("options")).Block(
				If(Id("requiresHTTP").Call(Id("option"))).Block(
					Id("serviceOptions").Op("=").Append(Id("serviceOptions"), Id("option")),
				).Else().Block(
					Id("configOptions").Op("=").Append(Id("configOptions"), Id("option")),
				),
			)
			bg.Line()
			bg.For(List(Id("_"), Id("option")).Op(":=").Range().Id("configOptions")).Block(
				Id("option").Call(Id("srv")),
			)
			bg.Line()
			bg.Id("srv").Dot("srvHTTP").Op("=").Qual(PackageFiber, "New").Call(Id("srv").Dot("config"))
			bg.Id("srv").Dot("srvHTTP").Dot("Use").Call(Id("recoverHandler"))
			if r.hasTrace() {
				bg.Id("srv").Dot("srvHTTP").Dot("Use").Call(Qual(fmt.Sprintf("%s/tracer", r.pkgPath(r.outDir)), "Middleware").Call())
			}
			bg.Id("srv").Dot("srvHTTP").Dot("Use").Call(Id("srv").Dot("setLogger"))
			bg.Id("srv").Dot("srvHTTP").Dot("Use").Call(Id("srv").Dot("headersHandler"))
			if r.hasJsonRPC() {
				bg.Id("srv").Dot("srvHTTP").Dot("Post").Call(Lit("/"), Id("srv").Dot("serveBatch"))
			}
			bg.Line()
			bg.For(List(Id("_"), Id("option")).Op(":=").Range().Id("serviceOptions")).Block(
				Id("option").Call(Id("srv")),
			)
			bg.Return()
		})
}

// serveHealthFunc генерирует функцию ServeHealth.
func (r *transportRenderer) serveHealthFunc() Code {

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	return Func().Id("ServeHealth").
		Params(
			Id("log").Op("*").Qual(PackageSlog, "Logger"),
			Id("path").String(),
			Id("address").String(),
			Id("response").Interface(),
		).
		Params(Op("*").Id("HealthServer")).
		BlockFunc(func(bg *Group) {
			bg.Var().Id("responseBody").Index().Byte()
			bg.Var().Err().Error()
			bg.If(Id("response").Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.List(Id("responseBody"), Err()).Op("=").Qual(jsonPkg, "Marshal").Call(Id("response"))
				ig.If(Err().Op("!=").Nil()).BlockFunc(func(eg *Group) {
					eg.Id("log").Dot("Error").Call(Lit("failed to marshal health response"), Qual(PackageSlog, "Any").Call(Lit("error"), Err()))
					eg.Id("responseBody").Op("=").Op("[]").Byte().Call(Lit(`{"status":"error","message":"health check misconfigured"}`))
				})
			}).Else().Block(
				Id("responseBody").Op("=").Op("[]").Byte().Call(Lit(`"ok"`)),
			)
			bg.Id("contentType").Op(":=").Id("contentTypeJson")
			bg.Id("contentLength").Op(":=").Len(Id("responseBody"))
			bg.Id("srv").Op(":=").Qual(PackageFiber, "New").Call(Qual(PackageFiber, "Config").Values(Dict{
				Id("DisableStartupMessage"): True(),
				Id("ReadTimeout"):           Id("defaultReadTimeout"),
				Id("WriteTimeout"):          Id("defaultWriteTimeout"),
				Id("IdleTimeout"):           Id("defaultIdleTimeout"),
			}))
			bg.Id("srv").Dot("Get").Call(Id("path"),
				Func().Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).Params(Error()).BlockFunc(func(hg *Group) {
					hg.Id(VarNameFtx).Dot("Response").Call().Dot("Header").Dot("SetContentType").Call(Id("contentType"))
					hg.Id(VarNameFtx).Dot("Response").Call().Dot("Header").Dot("SetContentLength").Call(Id("contentLength"))
					hg.List(Id("_"), Err()).Op("=").Id(VarNameFtx).Dot("Write").Call(Id("responseBody"))
					hg.Return(Err())
				}))
			bg.Go().Func().Params().Block(
				Err().Op(":=").Id("srv").Dot("Listen").Call(Id("address")),
				Id("ExitOnError").Call(Id("log"), Err(), Lit("serve health on ").Op("+").Id("address")),
			).Call()
			bg.Return(Op("&").Id("HealthServer").Values(Dict{
				Id("srv"):          Id("srv"),
				Id("responseBody"): Id("responseBody"),
			}))
		})
}

// shutdownFunc генерирует функцию Shutdown.
func (r *transportRenderer) shutdownFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("Shutdown").
		Params().
		Params(Id("err").Error()).
		BlockFunc(func(bg *Group) {
			bg.If(Id("srv").Dot("srvHTTP").Op("!=").Nil()).Block(
				If(Err().Op(":=").Id("srv").Dot("srvHTTP").Dot("ShutdownWithTimeout").Call(Id("defaultShutdownTimeout")).Op(";").Err().Op("!=").Nil()).Block(
					Return(Err()),
				),
			)
			if r.hasMetrics() {
				bg.If(Id("srv").Dot("srvMetrics").Op("!=").Nil()).Block(
					If(Err().Op(":=").Id("srv").Dot("srvMetrics").Dot("ShutdownWithTimeout").Call(Id("defaultShutdownTimeout")).Op(";").Err().Op("!=").Nil()).Block(
						Return(Err()),
					),
				)
			}
			bg.Return(Nil())
		})
}

// sendResponseFunc генерирует функцию sendResponse.
func (r *transportRenderer) sendResponseFunc() Code {

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	return Func().Id("sendResponse").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx"), Id("resp").Interface()).
		Params(Err().Error()).
		BlockFunc(func(bg *Group) {
			// Проверка на пустой batch ответ (только notifications)
			bg.If(List(Id("responses"), Id("ok")).Op(":=").Id("resp").Op(".").Call(Index().Op("*").Id("baseJsonRPC")).Op(";").Id("ok").Op("&&").Len(Id("responses")).Op("==").Lit(0)).Block(
				Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusNoContent")),
				Return(Nil()),
			)
			bg.Id(VarNameFtx).Dot("Response").Call().Dot("Header").Dot("SetContentType").Call(Id("contentTypeJson"))
			// Используем sync.Pool для буферов
			bg.Id("buf").Op(":=").Id("bufferPool").Dot("Get").Call().Op(".").Call(Op("*").Qual(PackageBytes, "Buffer"))
			bg.Defer().Func().Params().Block(
				Id("buf").Dot("Reset").Call(),
				Id("bufferPool").Dot("Put").Call(Id("buf")),
			).Call()
			bg.Id("encoder").Op(":=").Qual(jsonPkg, "NewEncoder").Call(Id("buf"))
			bg.If(Err().Op("=").Id("encoder").Dot("Encode").Call(Id("resp")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.If(Id("logger").Op(":=").Id("FromContext").Call(Id(VarNameFtx).Dot("UserContext").Call()).Op(";").Id("logger").Op("!=").Nil()).Block(
					Id("logger").Dot("Error").Call(Lit("response marshal error"), Qual(PackageSlog, "Any").Call(Lit("error"), Err())),
				)
				ig.Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusInternalServerError"))
				ig.Return(Err())
			})
			bg.List(Id("_"), Err()).Op("=").Id(VarNameFtx).Dot("Write").Call(Id("buf").Dot("Bytes").Call())
			bg.Return(Err())
		})
}

// sendHTTPErrorFunc генерирует функцию sendHTTPError.
func (r *transportRenderer) sendHTTPErrorFunc() Code {

	return Func().Id("sendHTTPError").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx"), Id("statusCode").Int(), Id("message").String()).
		Params(Err().Error()).
		BlockFunc(func(bg *Group) {
			bg.Id(VarNameFtx).Dot("Response").Call().Dot("Header").Dot("SetContentType").Call(Lit("text/plain"))
			bg.Id(VarNameFtx).Dot("Status").Call(Id("statusCode"))
			bg.List(Id("_"), Err()).Op("=").Id(VarNameFtx).Dot("WriteString").Call(Id("message"))
			bg.Return(Err())
		})
}

// httpServiceFunc генерирует функцию HTTPService.
func (r *transportRenderer) httpServiceFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("HTTPService").
		Params().
		Params(Op("*").Id("httpHTTPService")).
		Block(
			Return(Id("srv").Dot("httpHTTPService")),
		)
}

// requiresHTTPFunc генерирует функцию requiresHTTP.
func (r *transportRenderer) requiresHTTPFunc() Code {

	return Func().Id("requiresHTTP").
		Params(Id("option").Id("Option")).
		Params(Bool()).
		BlockFunc(func(bg *Group) {
			bg.Id("testSrv").Op(":=").Op("&").Id("Server").Do(func(s *Statement) {
				s.Add(Op("{"))
				s.Line()
				s.Id("headerHandlers").Op(":").Make(Map(String()).Id("HeaderHandler")).Op(",")
				s.Line()
				s.Add(Op("}"))
			})
			bg.Id("option").Call(Id("testSrv"))
			// Проверяем, что нет HTTP сервисов
			bg.Id("hasHTTPService").Op(":=").Id("testSrv").Dot("srvHTTP").Op("==").Nil()
			if r.hasHTTPService() {
				bg.Id("hasHTTPService").Op("=").Id("hasHTTPService").Op("&&").Id("testSrv").Dot("httpHTTPService").Op("==").Nil()
			}
			// Проверяем, что нет jsonRPC сервисов
			bg.Id("hasJsonRPCService").Op(":=").True()
			for _, contract := range r.project.Contracts {
				if contract.Annotations.Contains(TagServerJsonRPC) {
					bg.Id("hasJsonRPCService").Op("=").Id("hasJsonRPCService").Op("&&").Id("testSrv").Dot("http" + contract.Name).Op("==").Nil()
				}
			}
			bg.Return(Id("hasHTTPService").Op("&&").Id("hasJsonRPCService"))
		})
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderClientOptions генерирует файл options.go.
func (r *ClientRenderer) RenderClientOptions() error {

	outDir := r.outDir
	srcFile := NewSrcFile(filepath.Base(outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "jsonrpc")
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.ImportName(PackageHttp, "http")
		srcFile.ImportName(PackageTLS, "tls")
		srcFile.ImportName(PackageContext, "context")
		srcFile.ImportName(PackageSlog, "slog")
	}

	// Новая реализация - использует Client вместо ClientJsonRPC
	srcFile.Line().Type().Id("Option").Func().Params(Id("cli").Op("*").Id("Client"))

	srcFile.Line().Func().Params(Id("cli").Op("*").Id("Client")).Id("applyOpts").Params(Id("opts").Op("[]").Id("Option")).Block(
		For(List(Id("_"), Id("op")).Op(":=").Range().Id("opts")).Block(
			Id("op").Call(Id("cli")),
		),
	)

	srcFile.Line().Func().Id("DecodeError").Params(Id("decoder").Id("ErrorDecoder")).Params(Id("Option")).Block(
		Return(Func().Params(Id("cli").Op("*").Id("Client"))).Block(
			Id("cli").Dot("errorDecoder").Op("=").Id("decoder"),
		),
	)

	// Headers - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("Headers").Params(Id("headers").Op("...").Any()).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.Id("cli").Dot("headersFromCtx").Op("=").Append(Id("cli").Dot("headersFromCtx"), Id("headers").Op("..."))
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "HeaderFromCtx").Call(Id("headers").Op("...")))
				}
			})
		})
	}

	// ConfigTLS - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("ConfigTLS").Params(Id("tlsConfig").Op("*").Qual(PackageTLS, "Config")).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.If(Id("cli").Dot("httpClient").Op("!=").Nil()).Block(
					If(Id("transport").Op(",").Id("ok").Op(":=").Id("cli").Dot("httpClient").Dot("Transport").Assert(Op("*").Qual(PackageHttp, "Transport")).Op(";").Id("ok")).Block(
						Id("transport").Dot("TLSClientConfig").Op("=").Id("tlsConfig"),
					),
				)
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ConfigTLS").Call(Id("tlsConfig")))
				}
			})
		})
	}

	// LogRequest - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("LogRequest").Params().Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.Id("cli").Dot("logRequests").Op("=").True()
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "LogRequest").Call())
				}
			})
		})
	}

	// LogOnError - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("LogOnError").Params().Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.Id("cli").Dot("logOnError").Op("=").True()
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "LogOnError").Call())
				}
			})
		})
	}

	// ClientHTTP - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("ClientHTTP").Params(Id("client").Op("*").Qual(PackageHttp, "Client")).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.If(Id("cli").Dot("httpClient").Op("!=").Nil()).Block(
					Id("cli").Dot("httpClient").Op("=").Id("client"),
				)
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ClientHTTP").Call(Id("client")))
				}
			})
		})
	}

	// Transport - работает для JSON-RPC и HTTP, позволяет переопределить defaultTransport
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("Transport").Params(Id("transport").Qual(PackageHttp, "RoundTripper")).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.If(Id("cli").Dot("httpClient").Op("!=").Nil()).Block(
					Id("cli").Dot("httpClient").Dot("Transport").Op("=").Id("transport"),
				)
				if r.HasJsonRPC() {
					returnBg.If(Id("cli").Dot("httpClient").Op("!=").Nil()).Block(
						returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ClientHTTP").Call(Id("cli").Dot("httpClient"))),
					)
				}
			})
		})
	}

	// BeforeRequest - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("BeforeRequest").Params(Id("before").Func().Params(Qual(PackageContext, "Context"), Op("*").Qual(PackageHttp, "Request")).Params(Qual(PackageContext, "Context"))).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.Id("cli").Dot("beforeRequest").Op("=").Id("before")
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "BeforeRequest").Call(Id("before")))
				}
			})
		})
	}

	// AfterRequest - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("AfterRequest").Params(Id("after").Func().Params(Qual(PackageContext, "Context"), Op("*").Qual(PackageHttp, "Response")).Params(Err().Error())).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.Id("cli").Dot("afterRequest").Op("=").Id("after")
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "AfterRequest").Call(Id("after")))
				}
			})
		})
	}

	// AllowUnknownFields - работает для JSON-RPC и HTTP
	if r.HasJsonRPC() || r.HasHTTP() {
		srcFile.Line().Func().Id("AllowUnknownFields").Params(Id("allowUnknownFields").Bool()).Params(Id("Option")).BlockFunc(func(bg *Group) {
			bg.Return(Func().Params(Id("cli").Op("*").Id("Client"))).BlockFunc(func(returnBg *Group) {
				returnBg.Id("cli").Dot("allowUnknownFields").Op("=").Id("allowUnknownFields")
				if r.HasJsonRPC() {
					returnBg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "AllowUnknownFields").Call(Id("allowUnknownFields")))
				}
			})
		})
	}

	if r.HasMetrics() {
		srcFile.Line().Func().Id("WithMetrics").Params().Params(Id("Option")).Block(
			Return(Func().Params(Id("cli").Op("*").Id("Client"))).Block(
				If(Id("cli").Dot("metrics").Op("==").Nil()).Block(
					Id("cli").Dot("metrics").Op("=").Id("NewMetrics").Call(),
				),
			),
		)
	}
	return srcFile.Save(path.Join(outDir, "options.go"))
}


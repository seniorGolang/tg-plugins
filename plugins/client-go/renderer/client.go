// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/core"
)

// RenderClient генерирует базовый клиент с поддержкой JSON-RPC и HTTP
func (r *ClientRenderer) RenderClient() error {

	outDir := r.outDir

	// Копируем пакет jsonrpc (основа для клиента)
	if err := r.pkgCopyTo("jsonrpc", outDir); err != nil {
		return err
	}

	srcFile := NewSrcFile(filepath.Base(outDir))
	srcFile.PackageComment(DoNotEdit)
	srcFile.ImportName(PackageContext, "context")
	srcFile.ImportName(PackageHttp, "http")
	srcFile.ImportName(PackageOS, "os")
	srcFile.ImportName(PackageTime, "time")
	srcFile.ImportName(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "jsonrpc")

	// Генерируем структуру клиента
	srcFile.Line().Add(r.clientStructFunc(outDir))

	// Генерируем функцию New для инициализации клиента
	srcFile.Line().Func().Id("New").Params(Id("endpoint").String(), Id("opts").Op("...").Id("Option")).Params(Id("cli").Op("*").Id("Client")).BlockFunc(
		func(bg *Group) {
			bg.Line()
			bg.Var().Id("name").String()
			bg.List(Id("hostname"), Id("err")).Op(":=").Qual(PackageOS, "Hostname").Call()
			bg.If(Id("err").Op("==").Nil().Op("&&").Id("hostname").Op("!=").Lit("")).Block(
				Id("name").Op("=").Id("hostname").Op("+").Lit("_tg_").Op("+").Lit(r.project.Version),
			).Else().Block(
				Id("name").Op("=").Lit("tg_").Op("+").Lit(r.project.Version),
			)
			bg.Id("defaultTransport").Op(":=").Op("&").Qual(PackageHttp, "Transport").Values(Dict{
				Id("DisableKeepAlives"):     False(),
				Id("ExpectContinueTimeout"): Qual(PackageTime, "Second").Op("*").Lit(1),
				Id("ForceAttemptHTTP2"):     True(),
				Id("IdleConnTimeout"):       Qual(PackageTime, "Second").Op("*").Lit(60),
				Id("MaxConnsPerHost"):       Lit(1000),
				Id("MaxIdleConns"):          Lit(500),
				Id("MaxIdleConnsPerHost"):   Lit(100),
				Id("ResponseHeaderTimeout"): Qual(PackageTime, "Second").Op("*").Lit(10),
				Id("TLSHandshakeTimeout"):   Qual(PackageTime, "Second").Op("*").Lit(10),
			})
			bg.Id("defaultClient").Op(":=").Op("&").Qual(PackageHttp, "Client").Values(Dict{
				Id("Timeout"):   Qual(PackageTime, "Second").Op("*").Lit(30),
				Id("Transport"): Id("defaultTransport"),
			})
			bg.Id("cli").Op("=").Op("&").Id("Client").Values(DictFunc(func(dict Dict) {
				dict[Id("afterRequest")] = Nil()
				dict[Id("allowUnknownFields")] = False()
				dict[Id("beforeRequest")] = Nil()
				dict[Id("endpoint")] = Id("endpoint")
				dict[Id("errorDecoder")] = Id("defaultErrorDecoder")
				dict[Id("headersFromCtx")] = Index().Any().Values()
				dict[Id("httpClient")] = Id("defaultClient")
				dict[Id("logOnError")] = False()
				dict[Id("logRequests")] = False()
				dict[Id("name")] = Id("name")
			}))
			bg.Id("cli").Dot("applyOpts").Call(Id("opts"))
			if r.HasJsonRPC() {
				bg.Id("cli").Dot("rpcOpts").Op("=").Append(Id("cli").Dot("rpcOpts"), Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ClientHTTP").Call(Id("cli").Dot("httpClient")))
				bg.Id("cli").Dot("rpc").Op("=").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "NewClient").Call(Id("endpoint"), Id("cli").Dot("rpcOpts").Op("..."))
			}

			bg.Return()
		})

	// Генерируем методы для получения клиентов сервисов
	for _, contractName := range r.contractKeys() {
		contract := r.findContract(contractName)
		if contract == nil {
			continue
		}
		// Генерируем метод для сервиса, если он помечен как JSON-RPC или HTTP сервер
		if r.contains(contract.Annotations, TagServerJsonRPC) || r.contains(contract.Annotations, TagServerHTTP) {
			srcFile.Line().Func().Params(Id("cli").Op("*").Id("Client")).Id(contract.Name).Params().Params(Op("*").Id("Client" + contract.Name)).Block(
				Return(Op("&").Id("Client" + contract.Name).Values(Dict{
					Id("Client"): Id("cli"),
				})),
			)
		}
	}
	return srcFile.Save(path.Join(outDir, "client.go"))
}

// clientStructFunc генерирует структуру базового клиента
func (r *ClientRenderer) clientStructFunc(outDir string) Code {

	return Type().Id("Client").StructFunc(func(sg *Group) {
		sg.Id("name").String()
		sg.Id("endpoint").String()
		if r.HasJsonRPC() || r.HasHTTP() {
			sg.Line().Id("httpClient").Op("*").Qual(PackageHttp, "Client")
		}
		if r.HasJsonRPC() {
			sg.Line().Id("rpc").Op("*").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ClientRPC")
			sg.Id("rpcOpts").Op("[]").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "Option")
		}
		sg.Line().Id("errorDecoder").Id("ErrorDecoder")
		// Опции для HTTP и JSON-RPC
		if r.HasJsonRPC() || r.HasHTTP() {
			sg.Line().Id("logRequests").Bool()
			sg.Id("logOnError").Bool()
			sg.Id("headersFromCtx").Op("[]").Any()
			sg.Id("beforeRequest").Func().Params(Qual(PackageContext, "Context"), Op("*").Qual(PackageHttp, "Request")).Params(Qual(PackageContext, "Context"))
			sg.Id("afterRequest").Func().Params(Qual(PackageContext, "Context"), Op("*").Qual(PackageHttp, "Response")).Params(Err().Error())
		}
		if r.HasJsonRPC() || r.HasHTTP() {
			sg.Id("allowUnknownFields").Bool()
		}
		if r.HasMetrics() {
			sg.Line().Id("metrics").Op("*").Id("Metrics")
		}
	})
}

// findContract находит контракт по имени.
func (r *ClientRenderer) findContract(name string) *core.Contract {
	for _, contract := range r.project.Contracts {
		if contract.Name == name {
			return contract
		}
	}
	return nil
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderTransportJsonRPC генерирует транспортный JSON-RPC файл.
func (r *transportRenderer) RenderTransportJsonRPC() error {

	jsonrpcPath := path.Join(r.outDir, "jsonrpc.go")

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	r.renderJsonRPCImports(&srcFile)
	r.renderJsonRPCTypes(&srcFile)
	r.renderJsonRPCConstants(&srcFile)
	r.renderJsonRPCFunctions(&srcFile)

	return srcFile.Save(jsonrpcPath)
}

// renderJsonRPCImports генерирует импорты для JSON-RPC файла.
func (r *transportRenderer) renderJsonRPCImports(srcFile *GoFile) {

	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackageSync, "sync")
	srcFile.ImportName(PackageBytes, "bytes")
	srcFile.ImportName(PackageStrings, "strings")
	srcFile.ImportName(PackageErrors, "errors")
	srcFile.ImportName(PackageFmt, "fmt")
	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	srcFile.ImportName(jsonPkg, "json")
	contextPkgPath := fmt.Sprintf("%s/context", r.pkgPath(r.outDir))
	srcFile.ImportName(contextPkgPath, "context")
}

// renderJsonRPCTypes генерирует типы для JSON-RPC файла.
func (r *transportRenderer) renderJsonRPCTypes(srcFile *GoFile) {

	srcFile.Line().Add(r.jsonrpcConstants())
	srcFile.Add(r.idJsonRPC()).Line()
	srcFile.Add(r.baseJsonRPC()).Line()
	srcFile.Add(r.errorJsonRPC()).Line()
}

// renderJsonRPCConstants генерирует константы для JSON-RPC файла.
func (r *transportRenderer) renderJsonRPCConstants(srcFile *GoFile) {

	// Константы уже добавлены в renderJsonRPCTypes
}

// renderJsonRPCFunctions генерирует функции для JSON-RPC файла.
func (r *transportRenderer) renderJsonRPCFunctions(srcFile *GoFile) {

	srcFile.Add(r.jsonBufferPools())
	srcFile.Line()
	srcFile.Line().Type().Id("methodJsonRPC").
		Func().
		Params(Id(VarNameCtx).Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "Context"), Id("requestBase").Id("baseJsonRPC")).
		Params(Id("responseBase").Op("*").Id("baseJsonRPC"))
	srcFile.Line()
	srcFile.Line().Type().Id("methodJsonRPCWithFiber").
		Func().
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx"), Id("requestBase").Id("baseJsonRPC")).
		Params(Id("responseBase").Op("*").Id("baseJsonRPC"))
	srcFile.Line()
	srcFile.Line().Add(r.jsonRPCMethodMap())
	srcFile.Line()
	srcFile.Add(r.serveBatchFunc())
	srcFile.Add(r.batchFunc())
	srcFile.Add(r.singleBatchFunc())
	srcFile.Line()
	srcFile.Line()
	srcFile.Add(r.toLowercaseMethodFunc())
	srcFile.Add(r.sanitizeErrorMessageFunc())
	srcFile.Add(r.validateJsonRPCRequestFunc())
	srcFile.Line()
	srcFile.Line().Add(r.makeErrorResponseJsonRPCFunc())
}

// jsonrpcConstants генерирует константы для JSON-RPC.
func (r *transportRenderer) jsonrpcConstants() Code {

	return Const().Op("(").
		Line().Id("defaultMaxBatchSize").Op("=").Lit(100).
		Line().Id("defaultMaxParallelBatch").Op("=").Lit(10).
		Line().Id("Version").Op("=").Lit("2.0").
		Line().Id("contentTypeJson").Op("=").Lit("application/json").
		Line().Id("syncHeader").Op("=").Lit("X-Sync-On").
		Line().Id("parseError").Op("=").Lit(-32700).
		Line().Id("invalidRequestError").Op("=").Lit(-32600).
		Line().Id("methodNotFoundError").Op("=").Lit(-32601).
		Line().Id("invalidParamsError").Op("=").Lit(-32602).
		Line().Id("internalError").Op("=").Lit(-32603).
		Op(")")
}

// idJsonRPC генерирует тип idJsonRPC.
func (r *transportRenderer) idJsonRPC() Code {

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	return Type().Id("idJsonRPC").Op("=").Qual(jsonPkg, "RawMessage")
}

// baseJsonRPC генерирует тип baseJsonRPC.
func (r *transportRenderer) baseJsonRPC() Code {

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	return Type().Id("baseJsonRPC").StructFunc(func(tg *Group) {
		tg.Id("ID").Id("idJsonRPC").Tag(map[string]string{"json": "id"})
		tg.Id("Version").Id("string").Tag(map[string]string{"json": "jsonrpc"})
		tg.Id("Method").Id("string").Tag(map[string]string{"json": "method,omitempty"})
		tg.Id("Error").Op("*").Id("errorJsonRPC").Tag(map[string]string{"json": "error,omitempty"})
		tg.Id("Params").Qual(jsonPkg, "RawMessage").Tag(map[string]string{"json": "params,omitempty"})
		tg.Id("Result").Qual(jsonPkg, "RawMessage").Tag(map[string]string{"json": "result,omitempty"})
	})
}

// errorJsonRPC генерирует тип errorJsonRPC.
func (r *transportRenderer) errorJsonRPC() Code {

	return Type().Id("errorJsonRPC").Struct(
		Id("Code").Id("int").Tag(map[string]string{"json": "code"}),
		Id("Message").Id("string").Tag(map[string]string{"json": "message"}),
		Id("Data").Id("interface{}").Tag(map[string]string{"json": "data,omitempty"}),
	).Line().Func().Params(Err().Id("errorJsonRPC")).
		Id("Error").
		Params().
		Params(String()).
		Block(
			Return(Err().Dot("Message")),
		)
}

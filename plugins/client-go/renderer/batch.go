// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderClientBatch генерирует файл batch.go.
func (r *ClientRenderer) RenderClientBatch() error {

	outDir := r.outDir
	srcFile := NewSrcFile(filepath.Base(outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackageFmt, "fmt")
	srcFile.ImportName(PackageContext, "context")
	srcFile.ImportName(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "jsonrpc")

	srcFile.Line().Type().Id("RequestRPC").Struct(
		Id("retHandler").Id("rpcCallback"),
		Id("rpcRequest").Op("*").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "RequestRPC"),
	)

	srcFile.Line().Type().Id("rpcCallback").Func().Params(Err().Error(), Id("response").Op("*").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ResponseRPC"))

	srcFile.Line().Func().Params(Id("cli").Op("*").
		Id("Client")).Id("Batch").
		Params(Id(_ctx_).Qual(PackageContext, "Context"), Id("requests").Op("...").Id("RequestRPC")).BlockFunc(func(bg *Group) {
		bg.Line()
		bg.Var().Id("rpcRequests").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "RequestsRPC")
		bg.Id("callbacks").Op(":=").Make(Map(Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ID")).Id("rpcCallback"))
		bg.For(List(Id("_"), Id("request")).Op(":=").Range().Id("requests")).Block(
			Id("rpcRequests").Op("=").Append(Id("rpcRequests"), Id("request").Dot("rpcRequest")),
			Id("callbacks").Op("[").Id("request").Dot("rpcRequest").Dot("ID").Op("]").Op("=").Id("request").Dot("retHandler"),
		)
		bg.Var().Err().Error()
		bg.Var().Id("rpcResponses").Qual(fmt.Sprintf("%s/jsonrpc", r.pkgPath(outDir)), "ResponsesRPC")
		bg.List(Id("rpcResponses"), Err()).Op("=").Id("cli").Dot("rpc").Dot("CallBatch").Call(Id(_ctx_), Id("rpcRequests"))
		bg.If(Err().Op("!=").Nil().Op("||").Id("rpcResponses").Op("==").Nil()).Block(
			For(List(Id("_"), Id("callback")).Op(":=").Range().Id("callbacks")).Block(
				Id("callback").Call(Err(), Nil()),
			),
			Return(),
		)
		bg.For(List(Id("id"), Id("response")).Op(":=").Range().Id("rpcResponses").Dot("AsMap").Call()).Block(
			If(Id("callback").Op(":=").Id("callbacks").Op("[").Id("id").Op("]").Op(";").Id("callback").Op("!=").Nil()).Block(
				If(Id("response").Op("!=").Nil().Op("&&").Id("response").Dot("Error").Op("!=").Nil()).Block(
					Id("callback").Call(Qual(PackageFmt, "Errorf").Call(Lit("%s"), Id("response").Dot("Error").Dot("Message")), Id("response")),
				).Else().Block(
					Id("callback").Call(Nil(), Id("response")),
				),
			),
		)
	})
	return srcFile.Save(path.Join(outDir, "batch.go"))
}


// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderClientError генерирует файл error.go.
func (r *ClientRenderer) RenderClientError() error {

	outDir := r.outDir
	srcFile := NewSrcFile(filepath.Base(outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackageStdJSON, "json")

	srcFile.Add(r.errorJsonRPC())

	srcFile.Line().Type().Id("withErrorCode").Interface(
		Id("Code").Call().Int(),
	)

	srcFile.Line().Const().Id("internalError").Op("=").Lit(-32603)

	srcFile.Line().Type().Id("ErrorDecoder").Func().Params(Id("errData").Qual(PackageStdJSON, "RawMessage")).Params(Error())

	srcFile.Line().Func().Id("defaultErrorDecoder").Params(Id("errData").Qual(PackageStdJSON, "RawMessage")).Params(Err().Error()).Block(
		Line().Var().Id("jsonrpcError").Id("errorJsonRPC"),
		If(Err().Op("=").Qual(PackageStdJSON, "Unmarshal").Call(Id("errData"), Op("&").Id("jsonrpcError")).Op(";").Err().Op("!=").Nil()).Block(
			Return(),
		),
		Return(Id("jsonrpcError")),
	)

	return srcFile.Save(path.Join(outDir, "error.go"))
}

func (r *ClientRenderer) errorJsonRPC() Code {

	return Type().Id("errorJsonRPC").Struct(
		Id("Code").Id("int").Tag(map[string]string{"json": "code"}),
		Id("Message").Id("string").Tag(map[string]string{"json": "message"}),
		Id("Data").Any().Tag(map[string]string{"json": "data,omitempty"}),
	).Line().Func().Params(Err().Id("errorJsonRPC")).Id("Error").Params().Params(String()).Block(
		Return(Err().Dot("Message")),
	)
}


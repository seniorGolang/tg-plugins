// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderTransportErrors генерирует транспортный errors файл.
func (r *transportRenderer) RenderTransportErrors() error {

	errorsPath := path.Join(r.outDir, "errors.go")

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackageSlog, "slog")
	srcFile.ImportName("os", "os")

	srcFile.Line().Add(r.withErrorCodeInterface())
	srcFile.Line().Add(r.withRedirectInterface())
	srcFile.Line().Add(r.exitOnErrorFunc())

	return srcFile.Save(errorsPath)
}

// withErrorCodeInterface генерирует интерфейс withErrorCode.
func (r *transportRenderer) withErrorCodeInterface() Code {

	return Type().Id("withErrorCode").Interface(
		Id("Code").Params().Int(),
	)
}

// withRedirectInterface генерирует интерфейс withRedirect.
func (r *transportRenderer) withRedirectInterface() Code {

	return Type().Id("withRedirect").Interface(
		Id("RedirectTo").Params().String(),
	)
}

// exitOnErrorFunc генерирует функцию ExitOnError.
func (r *transportRenderer) exitOnErrorFunc() Code {

	return Func().Id("ExitOnError").
		Params(Id("log").Op("*").Qual(PackageSlog, "Logger"), Id("err").Error(), Id("msg").String()).
		Block(
			If(Id("err").Op("!=").Nil()).Block(
				Id("log").Dot("Error").Call(Id("msg"), Qual(PackageSlog, "Any").Call(Lit("error"), Id("err"))),
				Qual("os", "Exit").Call(Lit(1)),
			),
		)
}

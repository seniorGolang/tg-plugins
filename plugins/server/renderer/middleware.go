// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/renderer/types"
)

// RenderMiddleware генерирует типы middleware.
func (r *contractRenderer) RenderMiddleware() error {

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))

	typeGen := types.NewGenerator(r.project, &srcFile)

	for _, method := range r.contract.Methods {
		srcFile.Type().Id(r.contract.Name + method.Name).
			Func().
			Params(typeGen.FuncDefinitionParams(method.Args)).
			Params(typeGen.FuncDefinitionParams(method.Results))
	}

	srcFile.Line().Type().Id("Middleware" + r.contract.Name).
		Func().
		Params(Id("next").Qual(r.contract.PkgPath, r.contract.Name)).
		Params(Qual(r.contract.PkgPath, r.contract.Name)).
		Line()

	for _, method := range r.contract.Methods {
		srcFile.Type().Id("Middleware" + r.contract.Name + method.Name).
			Func().
			Params(Id("next").Id(r.contract.Name + method.Name)).
			Params(Id(r.contract.Name + method.Name))
	}

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-middleware.go"))
}

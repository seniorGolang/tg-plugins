// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
	"tgp/plugins/server/renderer/types"
)

// RenderTrace генерирует middleware для трейсинга.
func (r *contractRenderer) RenderTrace() error {

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))

	typeGen := types.NewGenerator(r.project, &srcFile)

	srcFile.Type().Id("trace" + r.contract.Name).Struct(
		Id("next").Qual(r.contract.PkgPath, r.contract.Name),
	)

	srcFile.Line().Func().Id("traceMiddleware" + r.contract.Name).
		Params(Id("next").Qual(r.contract.PkgPath, r.contract.Name)).
		Params(Qual(r.contract.PkgPath, r.contract.Name)).
		Block(
			Return(Op("&").Id("trace" + r.contract.Name).Values(Dict{
				Id("next"): Id("next"),
			})),
		)

	for _, method := range r.contract.Methods {
		srcFile.Line().Func().Params(Id("svc").Id("trace" + r.contract.Name)).
			Id(method.Name).
			Params(typeGen.FuncDefinitionParams(method.Args)).
			Params(typeGen.FuncDefinitionParams(method.Results)).
			BlockFunc(func(bg *Group) {
				bg.Line()
				bg.Var().Id("span").Qual(PackageTrace, "Span")
				bg.List(Id(VarNameCtx), Id("span")).Op("=").
					Qual(PackageOTEL, "Tracer").
					Call(Qual(PackageFmt, "Sprintf").Call(Lit("tg:%s"), Id("VersionTg"))).Dot("Start").Call(Id(VarNameCtx), Lit(r.methodFullName(method)))
				bg.Defer().Func().Params().Block(
					Id("span").Dot("RecordError").Call(Err()),
					Id("span").Dot("End").Call(),
				).Call()
				bg.Return(Id("svc").Dot("next").Dot(method.Name).CallFunc(func(cg *Group) {
					for _, arg := range method.Args {
						argCode := Id(arg.Name)
						if arg.IsEllipsis {
							argCode.Op("...")
						}
						cg.Add(argCode)
					}
				}))
			})
	}

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-trace.go"))
}

// methodFullName возвращает полное имя метода в формате "serviceName.methodName".
func (r *contractRenderer) methodFullName(method *core.Method) string {
	return fmt.Sprintf("%s.%s", toLowerCamel(r.contract.Name), toLowerCamel(method.Name))
}

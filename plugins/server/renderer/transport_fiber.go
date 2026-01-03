// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderTransportFiber генерирует транспортный fiber файл.
func (r *transportRenderer) RenderTransportFiber() error {

	fiberPath := path.Join(r.outDir, "fiber.go")

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackageErrors, "errors")
	srcFile.ImportName(PackageSlog, "slog")
	srcFile.ImportName(PackageContext, "context")
	srcFile.ImportName(PackageFmt, "fmt")

	srcFile.Line().Const().Id("logLevelHeader").Op("=").Lit("X-Log-Level")

	srcFile.Line().Type().Id("levelHandler").Struct(
		Id("handler").Qual(PackageSlog, "Handler"),
		Id("level").Op("*").Qual(PackageSlog, "LevelVar"),
	)
	srcFile.Line().Func().Params(Id("h").Op("*").Id("levelHandler")).
		Id("Enabled").
		Params(Id(VarNameCtx).Qual(PackageContext, "Context"), Id("level").Qual(PackageSlog, "Level")).
		Bool().
		Block(
			Return(Id("level").Op(">=").Id("h").Dot("level").Dot("Level").Call().Op("&&").Id("h").Dot("handler").Dot("Enabled").Call(Id(VarNameCtx), Id("level"))),
		)
	srcFile.Line().Func().Params(Id("h").Op("*").Id("levelHandler")).
		Id("Handle").
		Params(Id(VarNameCtx).Qual(PackageContext, "Context"), Id("r").Qual(PackageSlog, "Record")).
		Error().
		Block(
			Return(Id("h").Dot("handler").Dot("Handle").Call(Id(VarNameCtx), Id("r"))),
		)
	srcFile.Line().Func().Params(Id("h").Op("*").Id("levelHandler")).
		Id("WithAttrs").
		Params(Id("attrs").Index().Qual(PackageSlog, "Attr")).
		Qual(PackageSlog, "Handler").
		Block(
			Return(Op("&").Id("levelHandler").Values(Dict{
				Id("handler"): Id("h").Dot("handler").Dot("WithAttrs").Call(Id("attrs")),
				Id("level"):   Id("h").Dot("level"),
			})),
		)
	srcFile.Line().Func().Params(Id("h").Op("*").Id("levelHandler")).
		Id("WithGroup").
		Params(Id("name").String()).
		Qual(PackageSlog, "Handler").
		Block(
			Return(Op("&").Id("levelHandler").Values(Dict{
				Id("handler"): Id("h").Dot("handler").Dot("WithGroup").Call(Id("name")),
				Id("level"):   Id("h").Dot("level"),
			})),
		)

	r.renderFiberLogger(&srcFile)
	r.renderFiberRecover(&srcFile)

	return srcFile.Save(fiberPath)
}

// renderFiberLogger генерирует middleware для логирования в Fiber.
func (r *transportRenderer) renderFiberLogger(srcFile *GoFile) {

	srcFile.Line().Func().Params(Id("srv").Op("*").Id("Server")).
		Id("setLogger").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).
		Error().
		BlockFunc(func(bg *Group) {
			bg.Id("ctx").Op(":=").Id(VarNameFtx).Dot("UserContext").Call()
			bg.If(Id("FromContext").Call(Id("ctx")).Op("!=").Nil()).Block(
				Return(Id(VarNameFtx).Dot("Next").Call()),
			)
			bg.Id("levelName").Op(":=").String().Call(Id(VarNameFtx).Dot("Request").Call().Dot("Header").Dot("Peek").Call(Id("logLevelHeader")))
			bg.If(Id("levelName").Op("==").Lit("")).Block(
				Id(VarNameFtx).Dot("SetUserContext").Call(Id("WithLogger").Call(Id("ctx"), Id("srv").Dot("log"))),
				Return(Id(VarNameFtx).Dot("Next").Call()),
			)
			bg.Var().Id("level").Qual(PackageSlog, "Level")
			bg.Switch(Id("levelName")).Block(
				Case(Lit("debug"), Lit("DEBUG")).Block(Id("level").Op("=").Qual(PackageSlog, "LevelDebug")),
				Case(Lit("info"), Lit("INFO")).Block(Id("level").Op("=").Qual(PackageSlog, "LevelInfo")),
				Case(Lit("warn"), Lit("WARN")).Block(Id("level").Op("=").Qual(PackageSlog, "LevelWarn")),
				Case(Lit("error"), Lit("ERROR")).Block(Id("level").Op("=").Qual(PackageSlog, "LevelError")),
				Default().Block(
					Id(VarNameFtx).Dot("SetUserContext").Call(Id("WithLogger").Call(Id("ctx"), Id("srv").Dot("log"))),
					Return(Id(VarNameFtx).Dot("Next").Call()),
				),
			)
			bg.Id("levelVar").Op(":=").Op("new").Call(Qual(PackageSlog, "LevelVar"))
			bg.Id("levelVar").Dot("Set").Call(Id("level"))
			bg.Id("baseHandler").Op(":=").Id("srv").Dot("log").Dot("Handler").Call()
			bg.Id("requestLogger").Op(":=").Qual(PackageSlog, "New").Call(Op("&").Id("levelHandler").Values(Dict{
				Id("handler"): Id("baseHandler"),
				Id("level"):   Id("levelVar"),
			}))
			bg.Id(VarNameFtx).Dot("SetUserContext").Call(Id("WithLogger").Call(Id("ctx"), Id("requestLogger")))
			bg.Return(Id(VarNameFtx).Dot("Next").Call())
		})
}

// renderFiberRecover генерирует middleware для восстановления после panic в Fiber.
func (r *transportRenderer) renderFiberRecover(srcFile *GoFile) {

	srcFile.Line().Func().Id("recoverHandler").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).
		Error().
		Block(
			Defer().Func().Params().Block(
				If(Id("r").Op(":=").Recover().Op(";").Id("r").Op("!=").Nil().Block(
					List(Err(), Id("ok")).Op(":=").Id("r").Op(".").Call(Error()),
					If(Op("!").Id("ok")).Block(
						Err().Op("=").Qual(PackageErrors, "New").Call(Qual(PackageFmt, "Sprintf").Call(Lit("%v"), Id("r"))),
					),
					If(Id("logger").Op(":=").Id("FromContext").Call(Id(VarNameFtx).Dot("UserContext").Call()).Op(";").Id("logger").Op("!=").Nil()).Block(
						Id("logger").Dot("Error").Call(Lit("panic occurred"),
							Qual(PackageSlog, "Any").Call(Lit("error"), Qual(PackageErrors, "Wrap").Call(Err(), Lit("recover"))),
							Qual(PackageSlog, "String").Call(Lit("method"), Id(VarNameFtx).Dot("Method").Call()),
							Qual(PackageSlog, "String").Call(Lit("path"), Id(VarNameFtx).Dot("OriginalURL").Call()),
						),
					),
					Id(VarNameFtx).Dot("Status").Call(Qual(PackageFiber, "StatusInternalServerError")),
				)),
			).Call(),
			Return(Id(VarNameFtx).Dot("Next").Call()),
		)
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/internal/parser"
	"tgp/plugins/server/renderer/types"
)

// RenderLogger генерирует middleware для логирования.
func (r *contractRenderer) RenderLogger() error {

	if err := r.pkgCopyTo("viewer", r.outDir); err != nil {
		return fmt.Errorf("copy viewer package: %w", err)
	}

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackageSlog, "slog")
	srcFile.ImportName(PackageContext, "context")
	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))
	srcFile.ImportName(fmt.Sprintf("%s/viewer", r.pkgPath(r.outDir)), "viewer")

	typeGen := types.NewGenerator(r.project, &srcFile)

	// Генерируем константы для service и методов, чтобы избежать аллокаций
	srcFile.Line().Const().Id("logService" + r.contract.Name).Op("=").Lit(r.contract.Name)
	for _, method := range r.contract.Methods {
		srcFile.Const().Id("logMethod" + r.contract.Name + method.Name).Op("=").Lit(toLowerCamel(method.Name))
	}

	srcFile.Type().Id("logger" + r.contract.Name).Struct(
		Id(VarNameNext).Qual(r.contract.PkgPath, r.contract.Name),
	)

	srcFile.Line().Add(r.loggerMiddleware())

	for _, method := range r.contract.Methods {
		srcFile.Line().Func().Params(Id("m").Id("logger" + r.contract.Name)).
			Id(method.Name).
			Params(typeGen.FuncDefinitionParams(method.Args)).
			Params(typeGen.FuncDefinitionParams(method.Results)).
			BlockFunc(r.loggerFuncBody(method))
	}

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-logger.go"))
}

// loggerMiddleware генерирует функцию создания middleware для логирования.
func (r *contractRenderer) loggerMiddleware() Code {

	return Func().Id("loggerMiddleware" + r.contract.Name).
		Params().
		Params(Id("Middleware" + r.contract.Name)).
		Block(
			Return(Func().Params(Id(VarNameNext).Qual(r.contract.PkgPath, r.contract.Name)).Params(Qual(r.contract.PkgPath, r.contract.Name)).Block(
				Return(Op("&").Id("logger" + r.contract.Name).Values(Dict{
					Id(VarNameNext): Id(VarNameNext),
				})),
			)),
		)
}

// loggerFuncBody генерирует тело функции для метода с логированием.
func (r *contractRenderer) loggerFuncBody(method *parser.Method) func(bg *Group) {

	return func(bg *Group) {
		bg.Id("sLogger").Op(":=").Id("FromContext").Call(Id(VarNameCtx))
		bg.If(Id("sLogger").Op("==").Nil()).Block(
			Id("sLogger").Op("=").Qual(PackageSlog, "Default").Call(),
		)
		bg.Id("_begin").Op(":=").Qual(PackageTime, "Now").Call()
		bg.Defer().Func().Params().BlockFunc(func(bg *Group) {
			// Ленивое форматирование: проверяем уровень логирования перед форматированием
			bg.If(Op("!").Id("sLogger").Dot("Enabled").Call(Id(VarNameCtx), Qual(PackageSlog, "LevelInfo")).Op("&&").Id("err").Op("==").Nil()).Block(
				Return(), // Логирование отключено, не форматируем
			)
			bg.If(Op("!").Id("sLogger").Dot("Enabled").Call(Id(VarNameCtx), Qual(PackageSlog, "LevelError")).Op("&&").Id("err").Op("!=").Nil()).Block(
				Return(), // Логирование ошибок отключено
			)

			// Форматируем только если нужно логировать
			skipFields := strings.Split(method.Annotations.Value(TagLogSkip), ",")
			skipRequest := false
			skipResponse := false
			for _, field := range skipFields {
				field = strings.TrimSpace(field)
				if field == "request" {
					skipRequest = true
				}
				if field == "response" {
					skipResponse = true
				}
			}

			// Базовые attrs (всегда нужны)
			bg.Var().Id("attrs").Index().Qual(PackageSlog, "Attr")
			bg.Id("attrs").Op("=").Append(Id("attrs"),
				Qual(PackageSlog, "String").Call(Lit("service"), Id("logService"+r.contract.Name)),
				Qual(PackageSlog, "String").Call(Lit("method"), Id("logMethod"+r.contract.Name+method.Name)),
				Qual(PackageSlog, "String").Call(Lit("took"), Qual(PackageTime, "Since").Call(Id("_begin")).Dot("String").Call()),
			)

			// Обработка ошибки - ленивое форматирование только здесь
			bg.If(Id("err").Op("!=").Id("nil")).BlockFunc(func(bg *Group) {
				// Ленивое форматирование request только если не пропущен
				if !skipRequest {
					params := removeSkippedFields(r.ArgsFieldsWithoutContext(method), skipFields)
					originParams := removeSkippedFields(argsWithoutContext(method), skipFields)
					bg.Id("attrs").Op("=").Append(Id("attrs"), Qual(PackageSlog, "String").Call(Lit("request"), Qual(fmt.Sprintf("%s/viewer", r.pkgPath(r.outDir)), "Sprintf").Call(Lit("%+v"), Id(requestStructName(r.contract.Name, method.Name)).Values(r.dictByNormalVariables(params, originParams)))))
				}
				// Ленивое форматирование response только если не пропущен
				if !skipResponse {
					returns := r.ResultFieldsWithoutError(method)
					originReturns := resultsWithoutError(method)
					bg.Id("attrs").Op("=").Append(Id("attrs"), Qual(PackageSlog, "String").Call(Lit("response"), Qual(fmt.Sprintf("%s/viewer", r.pkgPath(r.outDir)), "Sprintf").Call(Lit("%+v"), Id(responseStructName(r.contract.Name, method.Name)).Values(r.dictByNormalVariables(returns, originReturns)))))
				}
				bg.Id("attrs").Op("=").Append(Id("attrs"), Qual(PackageSlog, "Any").Call(Lit("error"), Err()))
				bg.Var().Id("args").Index().Any()
				bg.For(List(Id("_"), Id("attr")).Op(":=").Range().Id("attrs")).Block(
					Id("args").Op("=").Append(Id("args"), Id("attr")),
				)
				bg.Id("sLogger").Dot("Error").Call(Lit(fmt.Sprintf("call %s", toLowerCamel(method.Name))), Id("args").Op("..."))
				bg.Return()
			})

			// Успешное выполнение - ленивое форматирование только здесь (не выполняется при ошибке)
			if !skipRequest {
				params := removeSkippedFields(r.ArgsFieldsWithoutContext(method), skipFields)
				originParams := removeSkippedFields(argsWithoutContext(method), skipFields)
				bg.Id("attrs").Op("=").Append(Id("attrs"), Qual(PackageSlog, "String").Call(Lit("request"), Qual(fmt.Sprintf("%s/viewer", r.pkgPath(r.outDir)), "Sprintf").Call(Lit("%+v"), Id(requestStructName(r.contract.Name, method.Name)).Values(r.dictByNormalVariables(params, originParams)))))
			}
			if !skipResponse {
				returns := r.ResultFieldsWithoutError(method)
				originReturns := resultsWithoutError(method)
				bg.Id("attrs").Op("=").Append(Id("attrs"), Qual(PackageSlog, "String").Call(Lit("response"), Qual(fmt.Sprintf("%s/viewer", r.pkgPath(r.outDir)), "Sprintf").Call(Lit("%+v"), Id(responseStructName(r.contract.Name, method.Name)).Values(r.dictByNormalVariables(returns, originReturns)))))
			}
			bg.Var().Id("args").Index().Any()
			bg.For(List(Id("_"), Id("attr")).Op(":=").Range().Id("attrs")).Block(
				Id("args").Op("=").Append(Id("args"), Id("attr")),
			)
			bg.Id("sLogger").Dot("Info").Call(Lit(fmt.Sprintf("call %s", toLowerCamel(method.Name))), Id("args").Op("..."))
		}).Call()
		bg.Return().Id("m").Dot(VarNameNext).Dot(method.Name).Call(r.paramNames(method.Args))
	}
}

// dictByNormalVariables создаёт Dict для jennifer из полей и нормальных переменных.
func (r *contractRenderer) dictByNormalVariables(fields []*parser.Variable, normals []*parser.Variable) Dict {

	if len(fields) != len(normals) {
		panic("len of fields and normals not the same")
	}
	return DictFunc(func(d Dict) {
		for i, field := range fields {
			normalVar := normals[i]
			normalVarCode := Id(toLowerCamel(normalVar.Name))

			// Если поле в структуре НЕ указатель, а нормальная переменная - указатель,
			// то нужно разыменовать указатель при использовании в struct literal
			if field.NumberOfPointers == 0 && normalVar.NumberOfPointers > 0 {
				normalVarCode = Op("*").Add(normalVarCode)
			}

			d[Id(toCamel(field.Name))] = normalVarCode
		}
	})
}

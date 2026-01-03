// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
	"tgp/plugins/server/renderer/types"
)

// RenderMetrics генерирует middleware для метрик.
func (r *contractRenderer) RenderMetrics() error {

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName("context", "context")
	srcFile.ImportName(PackageStrconv, "strconv")
	srcFile.ImportName(PackageTime, "time")
	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))
	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackagePrometheus, "metrics")

	typeGen := types.NewGenerator(r.project, &srcFile)

	// Генерируем константы для service и методов, чтобы избежать аллокаций
	srcFile.Line().Const().Defs(
		Id("metricService" + r.contract.Name).Op("=").Lit(toLowerCamel(r.contract.Name)),
	)
	for _, method := range r.contract.Methods {
		srcFile.Const().Id("metricMethod" + r.contract.Name + method.Name).Op("=").Lit(toLowerCamel(method.Name))
	}

	srcFile.Line().Type().Id("metrics"+r.contract.Name).Struct(
		Id(VarNameNext).Qual(r.contract.PkgPath, r.contract.Name),
		Id("metrics").Op("*").Id("Metrics"),
	)

	srcFile.Line().Add(r.metricsMiddleware())

	for _, method := range r.contract.Methods {
		srcFile.Line().Func().Params(Id("m").Id("metrics" + r.contract.Name)).
			Id(method.Name).
			Params(typeGen.FuncDefinitionParams(method.Args)).
			Params(typeGen.FuncDefinitionParams(method.Results)).
			BlockFunc(r.metricFuncBody(method))
	}

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-metrics.go"))
}

// metricsMiddleware генерирует функцию создания middleware для метрик.
func (r *contractRenderer) metricsMiddleware() Code {

	return Func().Id("metricsMiddleware"+r.contract.Name).
		Params(Id(VarNameNext).Qual(r.contract.PkgPath, r.contract.Name), Id("metrics").Op("*").Id("Metrics")).
		Params(Qual(r.contract.PkgPath, r.contract.Name)).
		BlockFunc(func(bg *Group) {
			bg.Return(Op("&").Id("metrics" + r.contract.Name).Values(
				Dict{
					Id(VarNameNext): Id(VarNameNext),
					Id("metrics"):   Id("metrics"),
				},
			))
		})
}

// metricFuncBody генерирует тело функции для метода с метриками.
func (r *contractRenderer) metricFuncBody(method *core.Method) func(bg *Group) {

	return func(bg *Group) {

		errCodeAssignment := Id("errCode").Op("=")

		if r.methodIsHTTP(method) {
			errCodeAssignment.Qual(PackageFiber, "StatusInternalServerError")
		} else {
			errCodeAssignment.Id("internalError")
		}

		bg.Line().Defer().Func().Params(Id("_begin").Qual(PackageTime, "Time")).Block(
			// Проверка на nil для метрик
			If(Id("m").Dot("metrics").Op("==").Nil()).Block(
				Return(),
			),
			Var().Defs(
				Id("success").Op("=").True(),
				Id("errCode").Int(),
			),
			If(Err().Op("!=").Nil()).Block(
				Id("success").Op("=").False(),
				errCodeAssignment,
				List(Id("ec"), Id("ok")).Op(":=").Err().Assert(Id("withErrorCode")),
				If(Id("ok")).Block(
					Id("errCode").Op("=").Id("ec").Dot("Code").Call(),
				),
			),
			// Оптимизация: для успешных запросов используем константы, для ошибок - форматируем только errCode
			If(Id("success")).Block(
				// Успешный запрос - используем константы
				Id("m").Dot("metrics").Dot("RequestCount").Dot("WithLabelValues").Call(
					Id("metricService"+r.contract.Name),
					Id("metricMethod"+r.contract.Name+method.Name),
					Id("metricSuccessTrue"),
					Id("metricErrCodeSuccess")).
					Dot("Add").Call(Lit(1)),
				Id("m").Dot("metrics").Dot("RequestCountAll").Dot("WithLabelValues").Call(
					Id("metricService"+r.contract.Name),
					Id("metricMethod"+r.contract.Name+method.Name),
					Id("metricSuccessTrue"),
					Id("metricErrCodeSuccess")).
					Dot("Add").Call(Lit(1)),
				Id("m").Dot("metrics").Dot("RequestLatency").Dot("WithLabelValues").Call(
					Id("metricService"+r.contract.Name),
					Id("metricMethod"+r.contract.Name+method.Name),
					Id("metricSuccessTrue"),
					Id("metricErrCodeSuccess")).
					Dot("Observe").Call(Id("float64").Call(Qual(PackageTime, "Since").Call(Id("_begin")).Dot("Microseconds").Call())),
			).Else().Block(
				// Ошибочный запрос - форматируем errCode
				Id("errCodeStr").Op(":=").Qual(PackageStrconv, "Itoa").Call(Id("errCode")),
				Id("m").Dot("metrics").Dot("RequestCount").Dot("WithLabelValues").Call(
					Id("metricService"+r.contract.Name),
					Id("metricMethod"+r.contract.Name+method.Name),
					Id("metricSuccessFalse"),
					Id("errCodeStr")).
					Dot("Add").Call(Lit(1)),
				Id("m").Dot("metrics").Dot("RequestCountAll").Dot("WithLabelValues").Call(
					Id("metricService"+r.contract.Name),
					Id("metricMethod"+r.contract.Name+method.Name),
					Id("metricSuccessFalse"),
					Id("errCodeStr")).
					Dot("Add").Call(Lit(1)),
				Id("m").Dot("metrics").Dot("RequestLatency").Dot("WithLabelValues").Call(
					Id("metricService"+r.contract.Name),
					Id("metricMethod"+r.contract.Name+method.Name),
					Id("metricSuccessFalse"),
					Id("errCodeStr")).
					Dot("Observe").Call(Id("float64").Call(Qual(PackageTime, "Since").Call(Id("_begin")).Dot("Microseconds").Call())),
			),
		).Call(Qual(PackageTime, "Now").Call())

		bg.Line().Return().Id("m").Dot(VarNameNext).Dot(method.Name).Call(r.paramNames(method.Args))
	}
}

// methodIsHTTP проверяет, является ли метод HTTP методом.
func (r *contractRenderer) methodIsHTTP(method *core.Method) bool {

	contractHasHTTP := r.contract.Annotations.Contains(TagServerHTTP)
	methodHasHTTP := method.Annotations.Contains(TagMethodHTTP)
	return contractHasHTTP && methodHasHTTP
}

// paramNames генерирует список имен параметров для вызова функции.
func (r *contractRenderer) paramNames(vars []*core.Variable) *Statement {

	var list = make([]Code, 0, len(vars))
	for _, v := range vars {
		paramCode := Id(toLowerCamel(v.Name))
		if v.IsEllipsis {
			paramCode.Op("...")
		}
		list = append(list, paramCode)
	}
	return List(list...)
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderClientMetrics генерирует файл metrics.go.
func (r *ClientRenderer) RenderClientMetrics() error {

	outDir := r.outDir
	srcFile := NewSrcFile(filepath.Base(outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName(PackagePrometheus, "prometheus")
	srcFile.ImportName(PackagePrometheusAuto, "promauto")
	srcFile.ImportName(PackageOS, "os")
	// VersionTg определен в version.go, который генерируется в том же пакете

	srcFile.Line().Type().Id("Metrics").Struct(
		Id("VersionGauge").Op("*").Qual(PackagePrometheus, "GaugeVec"),
		Id("RequestCount").Op("*").Qual(PackagePrometheus, "CounterVec"),
		Id("RequestCountAll").Op("*").Qual(PackagePrometheus, "CounterVec"),
		Id("RequestLatency").Op("*").Qual(PackagePrometheus, "HistogramVec"),
	).Line()

	srcFile.Line().Func().Id("NewMetrics").Params().Params(Op("*").Id("Metrics")).BlockFunc(func(bg *Group) {
		bg.List(Id("hostname"), Id("_")).Op(":=").Qual(PackageOS, "Hostname").Call()
		bg.Id("m").Op(":=").Op("&").Id("Metrics").Values(Dict{
			Id("VersionGauge"): Qual(PackagePrometheusAuto, "NewGaugeVec").Call(Qual(PackagePrometheus, "GaugeOpts").Values(
				DictFunc(func(d Dict) {
					d[Id("Name")] = Lit("count")
					d[Id("Namespace")] = Lit("client")
					d[Id("Subsystem")] = Lit("versions")
					d[Id("Help")] = Lit("Versions of client parts")
				}),
			), Index().String().Values(Lit("part"), Lit("version"), Lit("hostname"))),
			Id("RequestCount"): Qual(PackagePrometheusAuto, "NewCounterVec").Call(Qual(PackagePrometheus, "CounterOpts").Values(
				DictFunc(func(d Dict) {
					d[Id("Name")] = Lit("count")
					d[Id("Namespace")] = Lit("client")
					d[Id("Subsystem")] = Lit("requests")
					d[Id("Help")] = Lit("Number of requests sent")
				}),
			), Index().String().Values(Lit("service"), Lit("method"), Lit("success"), Lit("errCode"))),
			Id("RequestCountAll"): Qual(PackagePrometheusAuto, "NewCounterVec").Call(Qual(PackagePrometheus, "CounterOpts").Values(
				DictFunc(func(d Dict) {
					d[Id("Name")] = Lit("all_count")
					d[Id("Namespace")] = Lit("client")
					d[Id("Subsystem")] = Lit("requests")
					d[Id("Help")] = Lit("Number of all requests sent")
				}),
			), Index().String().Values(Lit("service"), Lit("method"), Lit("success"), Lit("errCode"))),
			Id("RequestLatency"): Qual(PackagePrometheusAuto, "NewHistogramVec").Call(Qual(PackagePrometheus, "HistogramOpts").Values(
				DictFunc(func(d Dict) {
					d[Id("Name")] = Lit("latency_microseconds")
					d[Id("Namespace")] = Lit("client")
					d[Id("Subsystem")] = Lit("requests")
					d[Id("Help")] = Lit("Total duration of requests in microseconds")
				}),
			), Index().String().Values(Lit("service"), Lit("method"), Lit("success"), Lit("errCode"))),
		})
		bg.Id("m").Dot("VersionGauge").Dot("WithLabelValues").Call(Lit("tg"), Id("VersionTg"), Id("hostname")).Dot("Set").Call(Lit(1))
		bg.Return(Id("m"))
	})

	return srcFile.Save(path.Join(outDir, "metrics.go"))
}


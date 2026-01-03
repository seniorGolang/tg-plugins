// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck
)

// RenderTransportMetrics генерирует транспортный metrics файл.
func (r *transportRenderer) RenderTransportMetrics() error {

	metricsPath := path.Join(r.outDir, "metrics.go")

	// Генерируем только если есть контракты с метриками
	hasMetrics := false
	for _, contract := range r.project.Contracts {
		if contract.Annotations.Contains(TagMetrics) {
			hasMetrics = true
			break
		}
	}
	if !hasMetrics {
		return nil
	}

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.ImportName("os", "os")
	srcFile.ImportName(PackagePrometheus, "prometheus")
	srcFile.ImportName("github.com/prometheus/client_golang/prometheus/promauto", "promauto")
	srcFile.ImportName(PackageSlog, "slog")
	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackageFiberAdaptor, "adaptor")
	srcFile.ImportName(PackagePrometheusHttp, "promhttp")

	srcFile.Line().Const().Defs(
		Id("metricSuccessTrue").Op("=").Lit("true"),
		Id("metricSuccessFalse").Op("=").Lit("false"),
		Id("metricErrCodeSuccess").Op("=").Lit("0"),
	)

	srcFile.Line().Type().Id("Metrics").Struct(
		Id("VersionGauge").Op("*").Qual(PackagePrometheus, "GaugeVec"),
		Id("RequestCount").Op("*").Qual(PackagePrometheus, "CounterVec"),
		Id("RequestCountAll").Op("*").Qual(PackagePrometheus, "CounterVec"),
		Id("RequestLatency").Op("*").Qual(PackagePrometheus, "HistogramVec"),
	)

	srcFile.Line().Add(r.newMetricsFunc())
	srcFile.Add(r.serveMetricsFunc())

	return srcFile.Save(metricsPath)
}

// newMetricsFunc генерирует функцию NewMetrics.
func (r *transportRenderer) newMetricsFunc() Code {

	return Func().Id("NewMetrics").
		Params().
		Params(Op("*").Id("Metrics")).
		BlockFunc(func(bg *Group) {
			bg.List(Id("hostname"), Id("_")).Op(":=").Qual("os", "Hostname").Call()
			bg.Id("m").Op(":=").Op("&").Id("Metrics").Values(Dict{
				Id("RequestCount"): Qual("github.com/prometheus/client_golang/prometheus/promauto", "NewCounterVec").Call(
					Qual(PackagePrometheus, "CounterOpts").Values(Dict{
						Id("Help"):      Lit("Number of requests received"),
						Id("Name"):      Lit("count"),
						Id("Namespace"): Lit("service"),
						Id("Subsystem"): Lit("requests"),
					}),
					Index().String().Values(Lit("service"), Lit("method"), Lit("success"), Lit("errCode")),
				),
				Id("RequestCountAll"): Qual("github.com/prometheus/client_golang/prometheus/promauto", "NewCounterVec").Call(
					Qual(PackagePrometheus, "CounterOpts").Values(Dict{
						Id("Help"):      Lit("Number of all requests received"),
						Id("Name"):      Lit("all_count"),
						Id("Namespace"): Lit("service"),
						Id("Subsystem"): Lit("requests"),
					}),
					Index().String().Values(Lit("service"), Lit("method"), Lit("success"), Lit("errCode")),
				),
				Id("RequestLatency"): Qual("github.com/prometheus/client_golang/prometheus/promauto", "NewHistogramVec").Call(
					Qual(PackagePrometheus, "HistogramOpts").Values(Dict{
						Id("Help"):      Lit("Total duration of requests in microseconds"),
						Id("Name"):      Lit("latency_microseconds"),
						Id("Namespace"): Lit("service"),
						Id("Subsystem"): Lit("requests"),
					}),
					Index().String().Values(Lit("service"), Lit("method"), Lit("success"), Lit("errCode")),
				),
				Id("VersionGauge"): Qual("github.com/prometheus/client_golang/prometheus/promauto", "NewGaugeVec").Call(
					Qual(PackagePrometheus, "GaugeOpts").Values(Dict{
						Id("Help"):      Lit("Versions of service parts"),
						Id("Name"):      Lit("count"),
						Id("Namespace"): Lit("service"),
						Id("Subsystem"): Lit("versions"),
					}),
					Index().String().Values(Lit("part"), Lit("version"), Lit("hostname")),
				),
			})
			bg.Id("m").Dot("VersionGauge").Dot("WithLabelValues").Call(Lit("tg"), Id("VersionTg"), Id("hostname")).Dot("Set").Call(Lit(1))
			bg.Return(Id("m"))
		})
}

// serveMetricsFunc генерирует функцию ServeMetrics.
func (r *transportRenderer) serveMetricsFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("ServeMetrics").
		Params(Id("log").Op("*").Qual(PackageSlog, "Logger"), Id("path").String(), Id("address").String()).
		Block(
			Id("srv").Dot("srvMetrics").Op("=").Qual(PackageFiber, "New").Call(Qual(PackageFiber, "Config").Values(Dict{Id("DisableStartupMessage"): True()})),
			Id("srv").Dot("srvMetrics").Dot("All").Call(Id("path"), Qual(PackageFiberAdaptor, "HTTPHandler").Call(Qual(PackagePrometheusHttp, "Handler").Call())),
			Go().Func().Params().Block(
				Err().Op(":=").Id("srv").Dot("srvMetrics").Dot("Listen").Call(Id("address")),
				Id("ExitOnError").Call(Id("log"), Err(), Lit("serve metrics on ").Op("+").Id("address")),
			).Call(),
		)
}

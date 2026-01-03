// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
)

// RenderTransportVersion генерирует транспортный version файл.
func (r *transportRenderer) RenderTransportVersion() error {

	versionPath := path.Join(r.outDir, "version.go")

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	// Версия берется из эталона transport_optimize
	srcFile.Const().Id("VersionTg").Op("=").Lit("v2.4.0")

	return srcFile.Save(versionPath)
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
)

// RenderVersion генерирует файл version.go.
func (r *ClientRenderer) RenderVersion() error {

	outDir := r.outDir
	srcFile := NewSrcFile(filepath.Base(outDir))
	srcFile.PackageComment(DoNotEdit)

	srcFile.Const().Id("VersionTg").Op("=").Lit(r.project.Version)

	return srcFile.Save(path.Join(outDir, "version.go"))
}


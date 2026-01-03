// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"path"
	"path/filepath"
)

// RenderTransportHTTP генерирует транспортный HTTP файл.
func (r *transportRenderer) RenderTransportHTTP() error {

	httpPath := path.Join(r.outDir, "http.go")

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	return srcFile.Save(httpPath)
}

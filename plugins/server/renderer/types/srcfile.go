// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package types

// SrcFile интерфейс для работы с файлом генерации кода.
type SrcFile interface {
	ImportName(pkgPath, name string)
	ImportAlias(pkgPath, alias string)
}

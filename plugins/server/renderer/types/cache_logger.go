// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package types

// CacheLogger интерфейс для логирования использования кэша.
type CacheLogger interface {
	OnCacheHit()
	OnCacheMiss()
}

var cacheLogger CacheLogger

// SetCacheLogger устанавливает логгер для отслеживания использования кэша.
func SetCacheLogger(logger CacheLogger) {

	cacheLogger = logger
}

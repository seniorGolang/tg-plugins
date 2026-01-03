// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package generator

import (
	"tgp/plugins/server/renderer/types"
)

// cacheLoggerImpl реализует types.CacheLogger для логирования использования кэша.
type cacheLoggerImpl struct{}

func (c *cacheLoggerImpl) OnCacheHit() {

	incrementCacheHits()
}

func (c *cacheLoggerImpl) OnCacheMiss() {

	incrementCacheMisses()
}

// setupCacheLogger настраивает логирование кэша для генератора типов.
func setupCacheLogger() {

	types.SetCacheLogger(&cacheLoggerImpl{})
}

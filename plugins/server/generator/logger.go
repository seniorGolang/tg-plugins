// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package generator

import (
	"fmt"
	"os"
	"sync/atomic"

	"tgp/shared"
)

var (
	verboseMode atomic.Bool
	stats       = &generationStats{}
)

// generationStats содержит статистику генерации.
type generationStats struct {
	filesGenerated int64
	linesGenerated int64
	cacheHits      int64
	cacheMisses    int64
}

// SetVerbose включает или выключает verbose режим логирования.
func SetVerbose(enabled bool) {

	verboseMode.Store(enabled)
	// В WASM плагине логирование управляется хостом
}

// IsVerbose возвращает, включен ли verbose режим.
func IsVerbose() bool {

	return verboseMode.Load()
}

// logVerbose логирует сообщение только в verbose режиме.
func logVerbose(msg string, args ...any) {

	if IsVerbose() {
		logger := shared.GetLogger()
		// Форматируем сообщение с аргументами
		formattedMsg := msg
		if len(args) > 0 {
			formattedMsg = fmt.Sprintf(msg, args...)
		}
		logger.Debug(formattedMsg)
	}
}

// incrementFilesGenerated увеличивает счетчик сгенерированных файлов.
func incrementFilesGenerated() {

	atomic.AddInt64(&stats.filesGenerated, 1)
}

// addLinesGenerated добавляет количество строк к статистике.
func addLinesGenerated(lines int64) {

	atomic.AddInt64(&stats.linesGenerated, lines)
}

// onFileSaved обрабатывает сохранение файла для статистики.
func onFileSaved(filepath string, lines int64) {

	incrementFilesGenerated()
	addLinesGenerated(lines)
	logVerbose("file generated: path=%s lines=%d", filepath, lines)
}

// incrementCacheHits увеличивает счетчик попаданий в кэш.
func incrementCacheHits() {

	atomic.AddInt64(&stats.cacheHits, 1)
}

// incrementCacheMisses увеличивает счетчик промахов кэша.
func incrementCacheMisses() {

	atomic.AddInt64(&stats.cacheMisses, 1)
}

// resetStats сбрасывает статистику.
func resetStats() {

	atomic.StoreInt64(&stats.filesGenerated, 0)
	atomic.StoreInt64(&stats.linesGenerated, 0)
	atomic.StoreInt64(&stats.cacheHits, 0)
	atomic.StoreInt64(&stats.cacheMisses, 0)
}

// logStats логирует статистику генерации.
func logStats() {

	logger := shared.GetLogger()
	files := atomic.LoadInt64(&stats.filesGenerated)
	lines := atomic.LoadInt64(&stats.linesGenerated)
	hits := atomic.LoadInt64(&stats.cacheHits)
	misses := atomic.LoadInt64(&stats.cacheMisses)

	totalCacheRequests := hits + misses
	cacheHitRate := float64(0)
	if totalCacheRequests > 0 {
		cacheHitRate = float64(hits) / float64(totalCacheRequests) * 100
	}

	logger.Info(fmt.Sprintf("generation statistics: files=%d lines=%d cache_hits=%d cache_misses=%d cache_hit_rate=%.2f%%",
		files, lines, hits, misses, cacheHitRate))
}

func init() {
	// Проверяем переменную окружения для verbose режима
	if os.Getenv("TG_VERBOSE") == "true" || os.Getenv("TG_VERBOSE") == "1" {
		SetVerbose(true)
	}
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"os"
	"path/filepath"

	"github.com/dave/jennifer/jen"

	"tgp/plugins/server/goimports"
)

// GoFile обертка над jen.File для генерации Go кода.
type GoFile struct {
	*jen.File
	filepath string
}

// NewSrcFile создает новый файл для генерации кода.
func NewSrcFile(pkgName string) GoFile {
	return GoFile{
		File: jen.NewFile(pkgName),
	}
}

// Save сохраняет сгенерированный код в файл и форматирует его через goimports.
func (src *GoFile) Save(filePath string) (err error) {

	src.filepath = filePath

	// Создаем директорию, если она не существует
	dir := filepath.Dir(filePath)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return
	}

	if err = src.File.Save(src.filepath); err != nil {
		return
	}

	var runner goimports.Runner
	if runner, err = goimports.NewFromFile(filePath); err != nil {
		return
	}

	if err = runner.Run(goimports.GetModulePath(filePath)); err != nil {
		return
	}

	// Подсчитываем строки в сгенерированном файле для статистики
	if lines, countErr := countLinesInFile(filePath); countErr == nil {
		// Вызываем callback для добавления статистики, если он установлен
		if onFileSaved != nil {
			onFileSaved(filePath, lines)
		}
	}

	return
}

// countLinesInFile подсчитывает количество строк в файле.
func countLinesInFile(filepath string) (int64, error) {

	content, err := os.ReadFile(filepath)
	if err != nil {
		return 0, err
	}

	lines := int64(1) // Минимум одна строка
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}
	return lines, nil
}

// onFileSavedCallback вызывается при сохранении файла для обновления статистики.
type onFileSavedCallback func(filepath string, lines int64)

var onFileSaved onFileSavedCallback

// SetOnFileSaved устанавливает callback для отслеживания сохраненных файлов.
func SetOnFileSaved(callback onFileSavedCallback) {

	onFileSaved = callback
}

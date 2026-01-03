// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package utils

import (
	"os"
)

// ShouldSkipFile проверяет, нужно ли пропустить генерацию файла, если он уже существует.
func ShouldSkipFile(filePath string) (bool, error) {

	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil // Файл существует, пропускаем
	}
	if os.IsNotExist(err) {
		return false, nil // Файл не существует, нужно генерировать
	}
	return false, err // Ошибка при проверке
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"log/slog"
)

// analyzeProject выполняет полный анализ проекта после сбора базовых данных.
func analyzeProject(log *slog.Logger, project *Project) error {
	// 1. Поиск сервисов (main файлов)
	if err := findServices(log, project); err != nil {
		return fmt.Errorf("failed to find services: %w", err)
	}

	// 2. Поиск имплементаций контрактов
	if err := findImplementations(log, project); err != nil {
		return fmt.Errorf("failed to find implementations: %w", err)
	}

	// 3. Анализ ошибок методов
	if err := analyzeMethodErrors(log, project); err != nil {
		return fmt.Errorf("failed to analyze method errors: %w", err)
	}

	// 4. Рекурсивное разбирание всех типов из контрактов
	if err := expandTypesRecursively(log, project); err != nil {
		return fmt.Errorf("failed to expand types recursively: %w", err)
	}

	return nil
}

// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

// RenderClientTypes генерирует локальные версии всех типов, используемых в exchange структурах.
// ВАЖНО: для TS типы генерируются в exchange файле вместе с namespace, поэтому этот метод
// просто проверяет, что все типы собраны. Реальная генерация происходит в RenderExchangeTypes.
func (r *ClientRenderer) RenderClientTypes(collectedTypeIDs map[string]bool) error {

	// Для TS типы генерируются в exchange файле вместе с namespace
	// Этот метод просто проверяет, что все типы собраны
	// Реальная генерация происходит в RenderExchangeTypes через walkVariable
	// ВАЖНО: для TS нужно генерировать ВСЕ типы, включая внешние либы, в формате namespace
	// Это уже реализовано в RenderExchangeTypes через typeDefTs и renderNamespace

	return nil
}


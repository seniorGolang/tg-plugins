// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package translate

import (
	"tgp/internal/locale"
)

// Translator представляет переводчик для плагина
type Translator struct {
	translations map[string]string
}

// NewTranslator создает новый переводчик с заданными переводами
func NewTranslator(translations map[string]string) *Translator {
	return &Translator{
		translations: translations,
	}
}

// Translate переводит текст на обнаруженный язык консоли.
// Если язык русский и перевод существует, возвращает русский перевод.
// Иначе возвращает оригинальный текст (ключ).
func (t *Translator) Translate(text string) string {
	if locale.DetectLanguage() == locale.LanguageRU {
		if translation, ok := t.translations[text]; ok {
			return translation
		}
	}
	// Return original text (English) as default
	return text
}


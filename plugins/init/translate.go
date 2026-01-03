// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package main

import (
	_ "embed"
	"encoding/json"
)

//go:embed translations/ru.json
var ruTranslationsJSON string

var (
	ruTranslations map[string]string
)

func init() {
	// Load Russian translations
	if err := json.Unmarshal([]byte(ruTranslationsJSON), &ruTranslations); err != nil {
		ruTranslations = make(map[string]string)
	}
}

// translate translates English text to the detected language.
// If language is Russian and translation exists, returns Russian translation.
// Otherwise, returns the original English text (key).
func translate(text string) string {
	// TODO: Добавить определение языка через хост, если нужно
	// Пока возвращаем русский перевод, если он есть
	if translation, ok := ruTranslations[text]; ok {
		return translation
	}
	// Return original text (English) as default
	return text
}

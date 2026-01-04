// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package main

import (
	_ "embed"
	"encoding/json"

	translatePkg "tgp/internal/translate"
)

//go:embed translations/ru.json
var ruTranslationsJSON string

var (
	translator *translatePkg.Translator
)

func init() {
	// Load Russian translations
	ruTranslations := make(map[string]string)
	if err := json.Unmarshal([]byte(ruTranslationsJSON), &ruTranslations); err != nil {
		ruTranslations = make(map[string]string)
	}
	translator = translatePkg.NewTranslator(ruTranslations)
}

// translate переводит текст на обнаруженный язык консоли
func translate(text string) string {
	return translator.Translate(text)
}

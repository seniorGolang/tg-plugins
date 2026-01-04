// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package locale

import (
	"os"
	"strings"
)

// Language represents supported languages
type Language string

const (
	LanguageEN Language = "en"
	LanguageRU Language = "ru"
)

// DetectLanguage detects console language from environment variables.
// Checks LC_ALL, LC_MESSAGES, and LANG in that order.
// Returns "ru" if Russian locale is detected, "en" otherwise.
func DetectLanguage() Language {
	// Check LC_ALL, LC_MESSAGES, and LANG in order
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		locale := os.Getenv(envVar)
		if locale == "" {
			continue
		}

		// Extract language code (e.g., "ru_RU.UTF-8" -> "ru")
		locale = strings.ToLower(locale)
		if strings.HasPrefix(locale, "ru") {
			return LanguageRU
		}
		if strings.HasPrefix(locale, "en") {
			return LanguageEN
		}
	}

	// Default to English if no locale is set
	return LanguageEN
}


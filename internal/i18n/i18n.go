// Package i18n provides the application's user-facing translations.
//
// Catalogs are embedded in the binary so releases remain self-contained. Add
// a JSON file under locales and its language code to supportedLanguages to add
// a translation.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	AutoLanguage    = "auto"
	DefaultLanguage = "en"
)

//go:embed locales/*.json
var localeFiles embed.FS

var supportedLanguages = map[string]struct{}{
	DefaultLanguage: {},
	"de":            {},
	"es":            {},
	"fr":            {},
	"it":            {},
	"pl":            {},
}

var catalogs = loadCatalogs()

type Localizer struct {
	language string
	messages map[string]string
}

func New(language string) Localizer {
	language = resolveLanguage(language, systemLanguage())
	messages := catalogs[language]
	if len(messages) == 0 {
		language = DefaultLanguage
		messages = catalogs[DefaultLanguage]
	}
	return Localizer{language: language, messages: messages}
}

func resolveLanguage(configuredLanguage, detectedLanguage string) string {
	configuredLanguage = strings.ToLower(strings.TrimSpace(configuredLanguage))
	if configuredLanguage == "" || configuredLanguage == AutoLanguage {
		return normalizeLanguage(detectedLanguage)
	}
	return normalizeLanguage(configuredLanguage)
}

func (l Localizer) Language() string {
	if l.language == "" {
		return DefaultLanguage
	}
	return l.language
}

func (l Localizer) Text(key string) string {
	if message := l.messages[key]; message != "" {
		return message
	}
	if message := catalogs[DefaultLanguage][key]; message != "" {
		return message
	}
	return key
}

func (l Localizer) Format(key string, args ...any) string {
	return fmt.Sprintf(l.Text(key), args...)
}

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	if _, ok := supportedLanguages[language]; ok {
		return language
	}
	if base, _, found := strings.Cut(language, "-"); found {
		if _, ok := supportedLanguages[base]; ok {
			return base
		}
	}
	return DefaultLanguage
}

func loadCatalogs() map[string]map[string]string {
	loaded := make(map[string]map[string]string, len(supportedLanguages))
	for language := range supportedLanguages {
		data, err := localeFiles.ReadFile("locales/" + language + ".json")
		if err != nil {
			continue
		}
		messages := make(map[string]string)
		if json.Unmarshal(data, &messages) == nil {
			loaded[language] = messages
		}
	}
	return loaded
}

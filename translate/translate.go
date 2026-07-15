package translate

import (
	"strings"
	"sync"
)

const DefaultLanguage = "eng"

type Translation struct {
	Language string
	Text     string
}

type Translations []Translation

var (
	mu       sync.RWMutex
	language = DefaultLanguage
)

func SetLanguage(lang string) {
	lang = normalize(lang)
	if lang == "" {
		lang = DefaultLanguage
	}

	mu.Lock()
	language = lang
	mu.Unlock()
}

func Language() string {
	mu.RLock()
	defer mu.RUnlock()
	return language
}

func Resolve(values Translations) string {
	return ResolveFor(Language(), values)
}

func ResolveFor(lang string, values Translations) string {
	lang = normalize(lang)
	if lang == "" {
		lang = DefaultLanguage
	}

	if text, ok := find(lang, values); ok {
		return text
	}
	if text, ok := find(DefaultLanguage, values); ok {
		return text
	}
	if len(values) > 0 {
		return values[0].Text
	}
	return ""
}

func find(lang string, values Translations) (string, bool) {
	for _, value := range values {
		if normalize(value.Language) == lang {
			return value.Text, true
		}
	}
	return "", false
}

func normalize(lang string) string {
	return strings.ToLower(strings.TrimSpace(lang))
}

package identify

import (
	"strings"

	"cinexplorer/internal/store"
	"cinexplorer/internal/tmdb"
)

// preferred lists, for a language, the regional variants to fall back on,
// closest first.
var preferred = map[string][]string{
	"es": {"es-MX", "es-ES"},
	"pt": {"pt-BR", "pt-PT"},
	"en": {"en-US", "en-GB"},
}

// Chain is the order in which translations are tried for lang: lang itself,
// then the preferred variants of its language ("es-AR" → es-AR, es-MX,
// es-ES). Any other variant of the language comes after the chain.
func Chain(lang string) []string {
	out := []string{lang}
	base, _, _ := strings.Cut(lang, "-")
	for _, v := range preferred[base] {
		if v != lang {
			out = append(out, v)
		}
	}
	return out
}

// pick returns the first non-empty text of the translations in the chain of
// lang, then of any variant of its language.
func pick(ts []tmdb.Translation, lang string, text func(tmdb.Translation) string) string {
	for _, tag := range Chain(lang) {
		for _, t := range ts {
			if t.Tag() == tag && text(t) != "" {
				return text(t)
			}
		}
	}
	base, _, _ := strings.Cut(lang, "-")
	for _, t := range ts {
		if t.Language == base && text(t) != "" {
			return text(t)
		}
	}
	return ""
}

func title(t tmdb.Translation) string    { return t.Data.Title }
func overview(t tmdb.Translation) string { return t.Data.Overview }

// translate completes a movie fetched in lang with its translations: title
// and overview from the chain of lang. A title found nowhere in the language
// stays as TMDB gave it (usually the original); an overview, and a title
// still empty, fall back to English.
func translate(m *store.Movie, ts []tmdb.Translation, lang string) {
	if t := pick(ts, lang, title); t != "" {
		m.Title = t
	}
	if o := pick(ts, lang, overview); o != "" {
		m.Overview = o
	}
	if m.Title == "" {
		m.Title = pick(ts, fallbackLang, title)
	}
	if m.Overview == "" {
		m.Overview = pick(ts, fallbackLang, overview)
	}
}

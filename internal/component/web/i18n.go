// Design: docs/architecture/web-interface.md -- web UI internationalization
// Related: render.go -- the "t" template helper is registered in NewRenderer's FuncMap
//
// i18n provides a minimal catalog-with-English-fallback translation layer for
// the web UI. English is the base locale: englishBase holds the default text
// for every key, and per-locale catalogs (e.g. French) override individual
// keys. A missing key or unknown locale falls back to English, and an entirely
// unknown key falls back to the key text itself, so a template never renders an
// empty string. The proving locale is French; the pipeline is what matters, not
// exhaustive coverage (spec A-7, R-6).

package web

import (
	"net/http"
	"strings"
)

const (
	// LocaleEnglish is the base locale; its text lives in englishBase.
	LocaleEnglish = "en"
	// LocaleFrench is the proving non-English locale.
	LocaleFrench = "fr"
)

// englishBase holds the default English text for every translation key.
var englishBase = map[string]string{
	"login.title":    "Login",
	"login.username": "Username",
	"login.password": "Password",
	"login.submit":   "Sign in",
}

// catalogs maps a non-English locale to its key->translation overrides.
// The "login.password" key is a UI label, not a credential (G101 false positive).
var catalogs = map[string]map[string]string{
	LocaleFrench: { //nolint:gosec // G101 false positive: translation keys are UI labels, not credentials
		"login.title":    "Connexion", //nolint:misspell // "Connexion" is correct French
		"login.username": "Nom d'utilisateur",
		"login.password": "Mot de passe",
		"login.submit":   "Se connecter",
	},
}

// Translate returns the translation of key for locale. It falls back to English
// when the locale or the key is missing from that locale's catalog, and to the
// key text itself when the key is unknown everywhere.
func Translate(locale, key string) string {
	if cat, ok := catalogs[locale]; ok {
		if v, ok := cat[key]; ok && v != "" {
			return v
		}
	}
	if v, ok := englishBase[key]; ok {
		return v
	}
	return key
}

// localeSupported reports whether a translation catalog exists for locale.
// English is always supported (it is the base).
func localeSupported(locale string) bool {
	if locale == LocaleEnglish {
		return true
	}
	_, ok := catalogs[locale]
	return ok
}

// LocaleFromRequest picks the UI locale from the request's Accept-Language
// header, returning the first supported locale or English.
func LocaleFromRequest(r *http.Request) string {
	if r == nil {
		return LocaleEnglish
	}
	return localeFromAcceptLanguage(r.Header.Get("Accept-Language"))
}

// localeFromAcceptLanguage parses an RFC 7231 Accept-Language header
// ("fr-FR,fr;q=0.9,en;q=0.8") and returns the first supported primary subtag,
// or English when none match.
func localeFromAcceptLanguage(header string) string {
	if header == "" {
		return LocaleEnglish
	}
	for part := range strings.SplitSeq(header, ",") {
		tag := strings.TrimSpace(part)
		if i := strings.IndexByte(tag, ';'); i >= 0 {
			tag = tag[:i] // drop the q-weight
		}
		tag = strings.ToLower(strings.TrimSpace(tag))
		if i := strings.IndexByte(tag, '-'); i >= 0 {
			tag = tag[:i] // primary subtag: fr-FR -> fr
		}
		if localeSupported(tag) {
			return tag
		}
	}
	return LocaleEnglish
}

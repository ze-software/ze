package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestI18NCatalogFallback is the AC-6 catalog contract: French keys translate,
// and anything missing falls back to English (then to the key text).
//
// VALIDATES: Translate resolves French, falls back to English for a missing
// key or unknown locale, and returns the key itself when it is unknown
// everywhere.
// PREVENTS: an empty/blank string ever rendering for an untranslated key.
func TestI18NCatalogFallback(t *testing.T) {
	// French catalog hit.
	assert.Equal(t, "Connexion", Translate(LocaleFrench, "login.title")) //nolint:misspell // correct French
	assert.Equal(t, "Se connecter", Translate(LocaleFrench, "login.submit"))

	// English base.
	assert.Equal(t, "Login", Translate(LocaleEnglish, "login.title"))
	assert.Equal(t, "Sign in", Translate(LocaleEnglish, "login.submit"))

	// Unknown locale falls back to English.
	assert.Equal(t, "Login", Translate("de", "login.title"))

	// Key present in English base but not in the French catalog: English fallback.
	// (Every current key is translated, so simulate via a base-only key.)
	englishBase["test.baseonly"] = "BaseOnly"
	t.Cleanup(func() { delete(englishBase, "test.baseonly") })
	assert.Equal(t, "BaseOnly", Translate(LocaleFrench, "test.baseonly"))

	// Unknown key everywhere: the key text is returned, never empty.
	assert.Equal(t, "totally.unknown", Translate(LocaleFrench, "totally.unknown"))
	assert.Equal(t, "totally.unknown", Translate(LocaleEnglish, "totally.unknown"))
}

// TestLocaleFromAcceptLanguage verifies Accept-Language negotiation picks the
// first supported locale and defaults to English.
func TestLocaleFromAcceptLanguage(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"fr-FR,fr;q=0.9,en;q=0.8", LocaleFrench},
		{"fr", LocaleFrench},
		{"en-US,en;q=0.9", LocaleEnglish},
		{"de-DE,de;q=0.9", LocaleEnglish}, // unsupported -> English
		{"", LocaleEnglish},
		{"de,fr;q=0.5", LocaleFrench}, // first supported wins over unsupported
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, localeFromAcceptLanguage(c.header), "header %q", c.header)
	}

	assert.Equal(t, LocaleEnglish, LocaleFromRequest(nil))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", http.NoBody)
	req.Header.Set("Accept-Language", "fr-CH,fr;q=0.9")
	assert.Equal(t, LocaleFrench, LocaleFromRequest(req))
}

// TestLoginTemplateRendersLocale proves the i18n pipeline end-to-end at the
// template tier: the login page renders French strings under the French locale
// and English otherwise, with the html lang attribute reflecting the locale.
func TestLoginTemplateRendersLocale(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)

	// French.
	recFR := httptest.NewRecorder()
	require.NoError(t, renderer.RenderLogin(recFR, LoginData{Locale: LocaleFrench}))
	fr := recFR.Body.String()
	assert.Contains(t, fr, "Connexion") //nolint:misspell // correct French
	// html/template escapes the apostrophe in "Nom d'utilisateur".
	assert.Contains(t, fr, "Nom d&#39;utilisateur")
	assert.Contains(t, fr, "Se connecter")
	assert.Contains(t, fr, `<html lang="fr">`)
	assert.False(t, strings.Contains(fr, "Sign in"), "French page must not carry the English submit label")

	// English (default).
	recEN := httptest.NewRecorder()
	require.NoError(t, renderer.RenderLogin(recEN, LoginData{Locale: LocaleEnglish}))
	en := recEN.Body.String()
	assert.Contains(t, en, "Login")
	assert.Contains(t, en, "Username")
	assert.Contains(t, en, "Sign in")
	assert.Contains(t, en, `<html lang="en">`)
}

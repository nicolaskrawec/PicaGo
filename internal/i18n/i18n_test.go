package i18n

import "testing"

func TestAllSupportedCatalogsAreComplete(t *testing.T) {
	want := catalogs[DefaultLanguage]
	for language := range supportedLanguages {
		catalog, ok := catalogs[language]
		if !ok {
			t.Errorf("catalog %q was not loaded", language)
			continue
		}
		for key := range want {
			if catalog[key] == "" {
				t.Errorf("catalog %q is missing %q", language, key)
			}
		}
		if len(catalog) != len(want) {
			t.Errorf("catalog %q has %d entries, want %d", language, len(catalog), len(want))
		}
	}
}

func TestEnglishCatalog(t *testing.T) {
	localizer := New("en-US")
	if got := localizer.Language(); got != "en" {
		t.Fatalf("language = %q, want en", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Zoom 125%" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestFrenchCatalog(t *testing.T) {
	localizer := New("fr-FR")
	if got := localizer.Language(); got != "fr" {
		t.Fatalf("language = %q, want fr", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Zoom 125 %" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestGermanCatalog(t *testing.T) {
	localizer := New("de-DE")
	if got := localizer.Language(); got != "de" {
		t.Fatalf("language = %q, want de", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Zoom 125 %" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestItalianCatalog(t *testing.T) {
	localizer := New("it-IT")
	if got := localizer.Language(); got != "it" {
		t.Fatalf("language = %q, want it", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Zoom 125%" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestSpanishCatalog(t *testing.T) {
	localizer := New("es-ES")
	if got := localizer.Language(); got != "es" {
		t.Fatalf("language = %q, want es", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Zoom 125 %" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestPolishCatalog(t *testing.T) {
	localizer := New("pl-PL")
	if got := localizer.Language(); got != "pl" {
		t.Fatalf("language = %q, want pl", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Powiększenie 125%" {
		t.Fatalf("formatted message = %q", got)
	}
}

func TestAutomaticLanguageUsesSupportedSystemLanguage(t *testing.T) {
	if got := resolveLanguage("auto", "fr-FR"); got != "fr" {
		t.Fatalf("language = %q, want fr", got)
	}
}

func TestAutomaticLanguageFallsBackToEnglish(t *testing.T) {
	if got := resolveLanguage("auto", "ja-JP"); got != "en" {
		t.Fatalf("language = %q, want en", got)
	}
}

func TestUnknownLanguageFallsBackToEnglish(t *testing.T) {
	localizer := New("unknown")
	if got := localizer.Text("state.none"); got != "none" {
		t.Fatalf("fallback message = %q, want none", got)
	}
}

func TestUnknownKeyIsVisible(t *testing.T) {
	if got := New("en").Text("missing.key"); got != "missing.key" {
		t.Fatalf("missing key = %q", got)
	}
}

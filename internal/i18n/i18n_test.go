package i18n

import "testing"

func TestEnglishCatalog(t *testing.T) {
	localizer := New("en-US")
	if got := localizer.Language(); got != "en" {
		t.Fatalf("language = %q, want en", got)
	}
	if got := localizer.Format("notification.zoom", 125); got != "Zoom 125%" {
		t.Fatalf("formatted message = %q", got)
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

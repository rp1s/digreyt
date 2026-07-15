package translate

import "testing"

func TestResolveUsesDefaultEnglish(t *testing.T) {
	SetLanguage("")
	got := Resolve(Translations{
		{Language: "ru", Text: "ошибка"},
		{Language: "eng", Text: "error"},
	})

	if got != "error" {
		t.Fatalf("expected english text, got %q", got)
	}
}

func TestResolveUsesSelectedLanguage(t *testing.T) {
	SetLanguage("ru")
	defer SetLanguage(DefaultLanguage)

	got := Resolve(Translations{
		{Language: "eng", Text: "error"},
		{Language: "ru", Text: "ошибка"},
	})

	if got != "ошибка" {
		t.Fatalf("expected russian text, got %q", got)
	}
}

func TestResolveSupportsCustomLanguage(t *testing.T) {
	SetLanguage("candy")
	defer SetLanguage(DefaultLanguage)

	got := Resolve(Translations{
		{Language: "eng", Text: "error"},
		{Language: "candy", Text: "sweet error"},
	})

	if got != "sweet error" {
		t.Fatalf("expected custom language text, got %q", got)
	}
}

func TestResolveFallsBackToEnglish(t *testing.T) {
	SetLanguage("missing")
	defer SetLanguage(DefaultLanguage)

	got := Resolve(Translations{
		{Language: "eng", Text: "error"},
		{Language: "ru", Text: "ошибка"},
	})

	if got != "error" {
		t.Fatalf("expected english fallback, got %q", got)
	}
}

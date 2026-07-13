package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestResolveAutoTranslatesMissingLanguage(t *testing.T) {
	prev := AutoTranslatorProvider()
	SetAutoTranslator(fakeTranslator{result: "ошибка"})
	t.Cleanup(func() { SetAutoTranslator(prev) })

	got, err := ResolveAutoFor(context.Background(), "ru", Translations{
		{Language: "eng", Text: "error"},
	})
	fmt.Println(got)
	if err != nil {
		t.Fatalf("ResolveAutoFor() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected automatic translation, got %q", got)
	}
}

func TestResolveAutoKeepsExistingLanguage(t *testing.T) {
	prev := AutoTranslatorProvider()
	SetAutoTranslator(fakeTranslator{result: "should not be used"})
	t.Cleanup(func() { SetAutoTranslator(prev) })

	got, err := ResolveAutoFor(context.Background(), "ru", Translations{
		{Language: "eng", Text: "error"},
		{Language: "ru", Text: "ошибка"},
	})
	if err != nil {
		t.Fatalf("ResolveAutoFor() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected existing translation, got %q", got)
	}
}

func TestMyMemoryTranslator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if got := query.Get("q"); got != "error" {
			t.Fatalf("q = %q, want error", got)
		}
		if got := query.Get("langpair"); got != "en|ru" {
			t.Fatalf("langpair = %q, want en|ru", got)
		}
		if got := query.Get("de"); got != "dev@example.com" {
			t.Fatalf("de = %q, want dev@example.com", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"responseData":{"translatedText":"ошибка"}}`))
	}))
	defer server.Close()

	translator := NewMyMemoryTranslator()
	translator.Endpoint = server.URL
	translator.Email = "dev@example.com"

	got, err := translator.Translate(context.Background(), "eng", "ru", "error")
	if err != nil {
		t.Fatalf("Translate() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected translated text, got %q", got)
	}
}

func TestMyMemoryTranslatorRejectsLongText(t *testing.T) {
	_, err := NewMyMemoryTranslator().Translate(context.Background(), "eng", "ru", strings.Repeat("x", 501))
	if err != ErrTranslationTooLong {
		t.Fatalf("expected ErrTranslationTooLong, got %v", err)
	}
}

func TestGoogleTranslator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		query := r.URL.Query()
		if got := query.Get("q"); got != "error" {
			t.Fatalf("q = %q, want error", got)
		}
		if got := query.Get("target"); got != "ru" {
			t.Fatalf("target = %q, want ru", got)
		}
		if got := query.Get("source"); got != "en" {
			t.Fatalf("source = %q, want en", got)
		}
		if got := query.Get("format"); got != "text" {
			t.Fatalf("format = %q, want text", got)
		}
		if got := query.Get("key"); got != "google-key" {
			t.Fatalf("key = %q, want google-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"translations":[{"translatedText":"ошибка"}]}}`))
	}))
	defer server.Close()

	translator := NewGoogleTranslator("google-key")
	translator.Endpoint = server.URL

	got, err := translator.Translate(context.Background(), "eng", "ru", "error")
	if err != nil {
		t.Fatalf("Translate() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected translated text, got %q", got)
	}
}

func TestGoogleTranslatorSupportsBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if got := r.URL.Query().Get("key"); got != "" {
			t.Fatalf("key = %q, want empty key with bearer auth", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"translations":[{"translatedText":"translated"}]}}`))
	}))
	defer server.Close()

	translator := &GoogleTranslator{
		Endpoint:    server.URL,
		BearerToken: "oauth-token",
		Format:      "text",
	}

	if _, err := translator.Translate(context.Background(), "eng", "ru", "error"); err != nil {
		t.Fatalf("Translate() failed: %v", err)
	}
}

func TestGoogleTranslatorRequiresToken(t *testing.T) {
	_, err := (&GoogleTranslator{}).Translate(context.Background(), "eng", "ru", "error")
	if err != ErrMissingToken {
		t.Fatalf("expected ErrMissingToken, got %v", err)
	}
}

func TestDeepLTranslator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "DeepL-Auth-Key deepl-key" {
			t.Fatalf("Authorization = %q, want DeepL auth key", got)
		}

		var body deepLRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if len(body.Text) != 1 || body.Text[0] != "error" {
			t.Fatalf("text = %#v, want [error]", body.Text)
		}
		if body.SourceLang != "EN" {
			t.Fatalf("source_lang = %q, want EN", body.SourceLang)
		}
		if body.TargetLang != "RU" {
			t.Fatalf("target_lang = %q, want RU", body.TargetLang)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translations":[{"text":"ошибка"}]}`))
	}))
	defer server.Close()

	translator := NewDeepLTranslator("deepl-key")
	translator.Endpoint = server.URL

	got, err := translator.Translate(context.Background(), "eng", "ru", "error")
	if err != nil {
		t.Fatalf("Translate() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected translated text, got %q", got)
	}
}

func TestDeepLTranslatorRequiresToken(t *testing.T) {
	_, err := NewDeepLTranslator("").Translate(context.Background(), "eng", "ru", "error")
	if err != ErrMissingToken {
		t.Fatalf("expected ErrMissingToken, got %v", err)
	}
}

func TestLibreTranslateTranslator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		var body libreTranslateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body.Q != "error" {
			t.Fatalf("q = %q, want error", body.Q)
		}
		if body.Source != "en" {
			t.Fatalf("source = %q, want en", body.Source)
		}
		if body.Target != "ru" {
			t.Fatalf("target = %q, want ru", body.Target)
		}
		if body.APIKey != "libre-key" {
			t.Fatalf("api_key = %q, want libre-key", body.APIKey)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translatedText":"ошибка"}`))
	}))
	defer server.Close()

	translator := NewLibreTranslateTranslator(server.URL, "libre-key")

	got, err := translator.Translate(context.Background(), "eng", "ru", "error")
	if err != nil {
		t.Fatalf("Translate() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected translated text, got %q", got)
	}
}

func TestTranslatorChainFallsBack(t *testing.T) {
	chain := NewTranslatorChain(
		fakeTranslator{err: errors.New("first provider failed")},
		fakeTranslator{result: "ошибка"},
	)

	got, err := chain.Translate(context.Background(), "eng", "ru", "error")
	if err != nil {
		t.Fatalf("Translate() failed: %v", err)
	}
	if got != "ошибка" {
		t.Fatalf("expected fallback translation, got %q", got)
	}
}

type fakeTranslator struct {
	result string
	err    error
}

func (f fakeTranslator) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.result, nil
}

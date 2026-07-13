package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const DefaultMyMemoryEndpoint = "https://api.mymemory.translated.net/get"
const DefaultGoogleTranslateEndpoint = "https://translation.googleapis.com/language/translate/v2"
const DefaultDeepLEndpoint = "https://api.deepl.com/v2/translate"
const DefaultDeepLFreeEndpoint = "https://api-free.deepl.com/v2/translate"
const DefaultLibreTranslateEndpoint = "https://libretranslate.com/translate"

var (
	ErrNoTranslator       = errors.New("translate: auto translator is not configured")
	ErrNoSourceText       = errors.New("translate: no source text to translate")
	ErrTranslationTooLong = errors.New("translate: text is longer than provider limit")
	ErrMissingToken       = errors.New("translate: provider token is required")
	ErrNoProviders        = errors.New("translate: no providers configured")
)

type AutoTranslator interface {
	Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error)
}

type TranslatorChain struct {
	Providers []AutoTranslator
}

type MyMemoryTranslator struct {
	Endpoint           string
	Email              string
	Key                string
	Client             *http.Client
	MachineTranslation bool
}

type GoogleTranslator struct {
	Endpoint    string
	APIKey      string
	BearerToken string
	Model       string
	Format      string
	Client      *http.Client
}

type DeepLTranslator struct {
	Endpoint string
	AuthKey  string
	Client   *http.Client
}

type LibreTranslateTranslator struct {
	Endpoint string
	APIKey   string
	Format   string
	Client   *http.Client
}

var (
	autoMu         sync.RWMutex
	autoTranslator AutoTranslator = NewMyMemoryTranslator()
)

func NewTranslatorChain(providers ...AutoTranslator) *TranslatorChain {
	return &TranslatorChain{Providers: providers}
}

func NewMyMemoryTranslator() *MyMemoryTranslator {
	return &MyMemoryTranslator{
		Endpoint:           DefaultMyMemoryEndpoint,
		Client:             &http.Client{Timeout: 10 * time.Second},
		MachineTranslation: true,
	}
}

func NewGoogleTranslator(apiKey string) *GoogleTranslator {
	return &GoogleTranslator{
		Endpoint: DefaultGoogleTranslateEndpoint,
		APIKey:   apiKey,
		Format:   "text",
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func NewDeepLTranslator(authKey string) *DeepLTranslator {
	return &DeepLTranslator{
		Endpoint: DefaultDeepLFreeEndpoint,
		AuthKey:  authKey,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func NewDeepLProTranslator(authKey string) *DeepLTranslator {
	return &DeepLTranslator{
		Endpoint: DefaultDeepLEndpoint,
		AuthKey:  authKey,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func NewLibreTranslateTranslator(endpoint, apiKey string) *LibreTranslateTranslator {
	if endpoint == "" {
		endpoint = DefaultLibreTranslateEndpoint
	}
	return &LibreTranslateTranslator{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Format:   "text",
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func SetAutoTranslator(translator AutoTranslator) {
	autoMu.Lock()
	autoTranslator = translator
	autoMu.Unlock()
}

func AutoTranslatorProvider() AutoTranslator {
	autoMu.RLock()
	defer autoMu.RUnlock()
	return autoTranslator
}

func ResolveAuto(ctx context.Context, values Translations) (string, error) {
	return ResolveAutoFor(ctx, Language(), values)
}

func ResolveAutoFor(ctx context.Context, lang string, values Translations) (string, error) {
	lang = normalize(lang)
	if lang == "" {
		lang = DefaultLanguage
	}

	if text, ok := find(lang, values); ok {
		return text, nil
	}

	source, ok := sourceForAutoTranslation(values)
	if !ok {
		return "", ErrNoSourceText
	}

	translator := AutoTranslatorProvider()
	if translator == nil {
		return "", ErrNoTranslator
	}

	return translator.Translate(ctx, source.Language, lang, source.Text)
}

func (t *TranslatorChain) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	if t == nil || len(t.Providers) == 0 {
		return "", ErrNoProviders
	}

	errs := make([]error, 0, len(t.Providers))
	for _, provider := range t.Providers {
		if provider == nil {
			continue
		}
		translated, err := provider.Translate(ctx, sourceLanguage, targetLanguage, text)
		if err == nil {
			return translated, nil
		}
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return "", ErrNoProviders
	}
	return "", errors.Join(errs...)
}

func (t *MyMemoryTranslator) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	if t == nil {
		return "", ErrNoTranslator
	}
	if len([]byte(text)) > 500 {
		return "", ErrTranslationTooLong
	}

	endpoint := strings.TrimSpace(t.Endpoint)
	if endpoint == "" {
		endpoint = DefaultMyMemoryEndpoint
	}

	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	query := reqURL.Query()
	query.Set("q", text)
	query.Set("langpair", apiLanguage(sourceLanguage)+"|"+apiLanguage(targetLanguage))
	if !t.MachineTranslation {
		query.Set("mt", "0")
	}
	if t.Email != "" {
		query.Set("de", t.Email)
	}
	if t.Key != "" {
		query.Set("key", t.Key)
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return "", err
	}

	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("translate: mymemory returned %s", res.Status)
	}

	var payload myMemoryResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.ResponseData.TranslatedText == "" {
		return "", fmt.Errorf("translate: mymemory returned empty translation")
	}

	return payload.ResponseData.TranslatedText, nil
}

func (t *GoogleTranslator) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	if t == nil {
		return "", ErrNoTranslator
	}
	if t.APIKey == "" && t.BearerToken == "" {
		return "", ErrMissingToken
	}

	endpoint := strings.TrimSpace(t.Endpoint)
	if endpoint == "" {
		endpoint = DefaultGoogleTranslateEndpoint
	}

	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	query := reqURL.Query()
	query.Set("q", text)
	query.Set("target", apiLanguage(targetLanguage))
	if source := apiLanguage(sourceLanguage); source != "" && source != "auto" {
		query.Set("source", source)
	}
	if t.Format != "" {
		query.Set("format", t.Format)
	}
	if t.Model != "" {
		query.Set("model", t.Model)
	}
	if t.APIKey != "" {
		query.Set("key", t.APIKey)
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), nil)
	if err != nil {
		return "", err
	}
	if t.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.BearerToken)
	}

	res, err := httpClient(t.Client).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("translate: google returned %s", res.Status)
	}

	var payload googleResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Data.Translations) == 0 || payload.Data.Translations[0].TranslatedText == "" {
		return "", fmt.Errorf("translate: google returned empty translation")
	}

	return html.UnescapeString(payload.Data.Translations[0].TranslatedText), nil
}

func (t *DeepLTranslator) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	if t == nil {
		return "", ErrNoTranslator
	}
	if t.AuthKey == "" {
		return "", ErrMissingToken
	}

	endpoint := strings.TrimSpace(t.Endpoint)
	if endpoint == "" {
		endpoint = DefaultDeepLFreeEndpoint
	}

	body := deepLRequest{
		Text:       []string{text},
		TargetLang: deepLLanguage(targetLanguage),
	}
	if source := deepLLanguage(sourceLanguage); source != "" && source != "AUTO" {
		body.SourceLang = source
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DeepL-Auth-Key "+t.AuthKey)

	res, err := httpClient(t.Client).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("translate: deepl returned %s", res.Status)
	}

	var payload deepLResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Translations) == 0 || payload.Translations[0].Text == "" {
		return "", fmt.Errorf("translate: deepl returned empty translation")
	}

	return payload.Translations[0].Text, nil
}

func (t *LibreTranslateTranslator) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	if t == nil {
		return "", ErrNoTranslator
	}

	endpoint := strings.TrimSpace(t.Endpoint)
	if endpoint == "" {
		endpoint = DefaultLibreTranslateEndpoint
	}

	body := libreTranslateRequest{
		Q:      text,
		Source: apiLanguage(sourceLanguage),
		Target: apiLanguage(targetLanguage),
		Format: t.Format,
		APIKey: t.APIKey,
	}
	if body.Source == "" {
		body.Source = "auto"
	}
	if body.Format == "" {
		body.Format = "text"
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := httpClient(t.Client).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("translate: libretranslate returned %s", res.Status)
	}

	var payload libreTranslateResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TranslatedText == "" {
		return "", fmt.Errorf("translate: libretranslate returned empty translation")
	}

	return payload.TranslatedText, nil
}

func sourceForAutoTranslation(values Translations) (Translation, bool) {
	if source, ok := findTranslation(DefaultLanguage, values); ok {
		return source, true
	}
	if len(values) > 0 {
		return values[0], true
	}
	return Translation{}, false
}

func findTranslation(lang string, values Translations) (Translation, bool) {
	for _, value := range values {
		if normalize(value.Language) == normalize(lang) {
			return value, true
		}
	}
	return Translation{}, false
}

func apiLanguage(lang string) string {
	switch normalize(lang) {
	case "eng":
		return "en"
	case "rus":
		return "ru"
	case "ukr":
		return "uk"
	default:
		return normalize(lang)
	}
}

func deepLLanguage(lang string) string {
	return strings.ToUpper(apiLanguage(lang))
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

type myMemoryResponse struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
}

type googleResponse struct {
	Data struct {
		Translations []struct {
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	} `json:"data"`
}

type deepLRequest struct {
	Text       []string `json:"text"`
	TargetLang string   `json:"target_lang"`
	SourceLang string   `json:"source_lang,omitempty"`
}

type deepLResponse struct {
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
}

type libreTranslateRequest struct {
	Q      string `json:"q"`
	Source string `json:"source"`
	Target string `json:"target"`
	Format string `json:"format,omitempty"`
	APIKey string `json:"api_key,omitempty"`
}

type libreTranslateResponse struct {
	TranslatedText string `json:"translatedText"`
}

package digreyt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/rp1s/digreyt/translate"
)

func TestPrintDiagnostics(t *testing.T) {
	arena := &Arena{
		Source: `fn main() {
print("Hello, World!")
}`}

	arena.Add(Error{
		CodeName:      "TestError",
		Message:       "test diagnostic",
		Arrow:         "look here",
		Severity:      SeverityError,
		IsShowSnippet: true,
		Pos: Position{
			FileName: "main.fg",
			Line:     1,
			Column:   1,
			Offset:   0,
		},
		Start: 0,
		End:   2,
	})

	arena.Add(Error{
		CodeName: "TestWarning",
		Message:  "test warning",
		Severity: SeverityWarning,
		Pos: Position{
			FileName: "main.fg",
			Line:     2,
			Column:   1,
		},
		Start: 0,
		End:   5,
	})

	if !arena.HasErrors() {
		t.Errorf("Expected arena to have errors, but it does not.")
	}

	var out bytes.Buffer
	if err := arena.Render(&out); err != nil {
		t.Fatalf("Render() failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "TestError") {
		t.Fatalf("rendered diagnostics do not contain error code: %q", got)
	}
	if !strings.Contains(got, "TestWarning") {
		t.Fatalf("rendered diagnostics do not contain warning code: %q", got)
	}

	arena.Clear()

	if arena.HasErrors() {
		t.Errorf("Expected arena to be cleared of errors, but it still has errors.")
	}
}

func TestCustomRendererCanReplaceView(t *testing.T) {
	arena := New("ok")
	arena.Add(Error{
		CodeName: "Done",
		Message:  "compiled",
		Severity: SeveritySuccess,
	})

	var out bytes.Buffer
	err := arena.PrintWith(&out, RendererFunc(func(w io.Writer, source string, err Error) error {
		_, writeErr := fmt.Fprintf(w, "%s:%s:%s", err.Severity, err.CodeName, source)
		return writeErr
	}))
	if err != nil {
		t.Fatalf("PrintWith() failed: %v", err)
	}

	if got, want := out.String(), "success:Done:ok"; got != want {
		t.Fatalf("custom render mismatch:\ngot:  %q\nwant: %q", got, want)
	}
	if arena.HasErrors() {
		t.Fatal("PrintWith() must clear rendered diagnostics")
	}
}

func TestAddErrorConvertsPlainAndDiagnosticErrors(t *testing.T) {
	arena := New("")
	if ok := arena.AddError(errors.New("plain failure")); !ok {
		t.Fatal("AddError() rejected plain error")
	}

	complex := complexDiagnostic{message: "careful", severity: SeverityWarning}
	if ok := arena.AddError(fmt.Errorf("wrapped: %w", complex)); !ok {
		t.Fatal("AddError() rejected diagnostic error")
	}

	if got := len(arena.Errors); got != 2 {
		t.Fatalf("unexpected diagnostic count: %d", got)
	}
	if got := arena.Errors[0].Severity; got != SeverityError {
		t.Fatalf("plain error severity = %s, want error", got)
	}
	if got := arena.Errors[1].Severity; got != SeverityWarning {
		t.Fatalf("complex error severity = %s, want warning", got)
	}
}

func TestRenderAutoLocalizesDiagnostics(t *testing.T) {
	prevLanguage := translate.Language()
	prevTranslator := translate.AutoTranslatorProvider()
	translate.SetLanguage("ru")
	translate.SetAutoTranslator(autoFakeTranslator{result: "неожиданный токен"})
	t.Cleanup(func() {
		translate.SetLanguage(prevLanguage)
		translate.SetAutoTranslator(prevTranslator)
	})

	arena := New("")
	arena.Add(Error{
		CodeName: "ParseError",
		MessageTranslations: translate.Translations{
			{Language: "eng", Text: "unexpected token"},
		},
	})

	var out bytes.Buffer
	if err := arena.RenderAuto(context.Background(), &out); err != nil {
		t.Fatalf("RenderAuto() failed: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "неожиданный токен") {
		t.Fatalf("rendered output does not contain auto translation: %q", got)
	}
}

type complexDiagnostic struct {
	message  string
	severity Severity
}

func (d complexDiagnostic) Error() string {
	return d.message
}

func (d complexDiagnostic) AsDiagnostic() Error {
	return Error{
		CodeName: "Complex",
		Message:  d.message,
		Severity: d.severity,
	}
}

type autoFakeTranslator struct {
	result string
}

func (f autoFakeTranslator) Translate(ctx context.Context, sourceLanguage, targetLanguage, text string) (string, error) {
	return f.result, nil
}

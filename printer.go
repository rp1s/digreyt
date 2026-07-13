// Package digreyt
//
// Здесь допускается использование более тяжёлых по ресурсам решений,
// поскольку этот пакет вызывается только при возникновении ошибок.
package digreyt

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rp1s/colorista"
)

const contextLines = 5

type Renderer interface {
	Render(w io.Writer, source string, err Error) error
}

type RendererFunc func(w io.Writer, source string, err Error) error

func (f RendererFunc) Render(w io.Writer, source string, err Error) error {
	return f(w, source, err)
}

type RenderTheme struct {
	ContextLines      int
	LocationArrow     string
	ModuleLabel       string
	DescriptionBullet string
	MutedStyles       []colorista.Style
	LocationStyles    []colorista.Style
	SeverityViews     map[Severity]SeverityView
}

type SeverityView struct {
	Symbol            string
	Label             string
	SymbolStyles      []colorista.Style
	CodeStyles        []colorista.Style
	CaretStyles       []colorista.Style
	ArrowStyles       []colorista.Style
	DescriptionStyles []colorista.Style
	BulletStyles      []colorista.Style
}

type ColorRenderer struct {
	Colorista *colorista.Colorista
	Theme     RenderTheme
}

func DefaultRenderer() *ColorRenderer {
	return NewColorRenderer(DefaultRenderTheme())
}

func NewColorRenderer(theme RenderTheme) *ColorRenderer {
	return &ColorRenderer{
		Colorista: colorista.NewColorista(colorista.ThemeAuto),
		Theme:     theme.withDefaults(),
	}
}

func DefaultRenderTheme() RenderTheme {
	return RenderTheme{
		ContextLines:      contextLines,
		LocationArrow:     "──> ",
		ModuleLabel:       "Module: ",
		DescriptionBullet: "• ",
		MutedStyles:       []colorista.Style{colorista.BrightBlack},
		LocationStyles:    []colorista.Style{colorista.Bold, colorista.BrightBlue},
		SeverityViews: map[Severity]SeverityView{
			SeverityError: {
				Symbol:            "×",
				Label:             "error",
				SymbolStyles:      []colorista.Style{colorista.Bold, colorista.BrightRed},
				CodeStyles:        []colorista.Style{colorista.Bold, colorista.BrightRed},
				CaretStyles:       []colorista.Style{colorista.Bold, colorista.BrightRed},
				ArrowStyles:       []colorista.Style{colorista.Rgb(colorista.RGB{R: 255, G: 221, B: 120})},
				BulletStyles:      []colorista.Style{colorista.Bold, colorista.BrightGreen},
				DescriptionStyles: []colorista.Style{},
			},
			SeverityWarning: {
				Symbol:            "!",
				Label:             "warning",
				SymbolStyles:      []colorista.Style{colorista.Bold, colorista.BrightYellow},
				CodeStyles:        []colorista.Style{colorista.Bold, colorista.BrightYellow},
				CaretStyles:       []colorista.Style{colorista.Bold, colorista.BrightYellow},
				ArrowStyles:       []colorista.Style{colorista.Bold, colorista.BrightYellow},
				BulletStyles:      []colorista.Style{colorista.Bold, colorista.BrightYellow},
				DescriptionStyles: []colorista.Style{},
			},
			SeverityInfo: {
				Symbol:            "i",
				Label:             "info",
				SymbolStyles:      []colorista.Style{colorista.Bold, colorista.BrightBlue},
				CodeStyles:        []colorista.Style{colorista.Bold, colorista.BrightBlue},
				CaretStyles:       []colorista.Style{colorista.Bold, colorista.BrightBlue},
				ArrowStyles:       []colorista.Style{colorista.Bold, colorista.BrightBlue},
				BulletStyles:      []colorista.Style{colorista.Bold, colorista.BrightBlue},
				DescriptionStyles: []colorista.Style{},
			},
			SeveritySuccess: {
				Symbol:            "✓",
				Label:             "success",
				SymbolStyles:      []colorista.Style{colorista.Bold, colorista.BrightGreen},
				CodeStyles:        []colorista.Style{colorista.Bold, colorista.BrightGreen},
				CaretStyles:       []colorista.Style{colorista.Bold, colorista.BrightGreen},
				ArrowStyles:       []colorista.Style{colorista.Bold, colorista.BrightGreen},
				BulletStyles:      []colorista.Style{colorista.Bold, colorista.BrightGreen},
				DescriptionStyles: []colorista.Style{},
			},
		},
	}
}

func (t RenderTheme) withDefaults() RenderTheme {
	defaults := DefaultRenderTheme()
	if t.ContextLines == 0 {
		t.ContextLines = defaults.ContextLines
	}
	if t.LocationArrow == "" {
		t.LocationArrow = defaults.LocationArrow
	}
	if t.ModuleLabel == "" {
		t.ModuleLabel = defaults.ModuleLabel
	}
	if t.DescriptionBullet == "" {
		t.DescriptionBullet = defaults.DescriptionBullet
	}
	if t.MutedStyles == nil {
		t.MutedStyles = defaults.MutedStyles
	}
	if t.LocationStyles == nil {
		t.LocationStyles = defaults.LocationStyles
	}
	if t.SeverityViews == nil {
		t.SeverityViews = defaults.SeverityViews
		return t
	}
	for severity, view := range defaults.SeverityViews {
		if _, ok := t.SeverityViews[severity]; !ok {
			t.SeverityViews[severity] = view
		}
	}
	return t
}

type Arena struct {
	Source   string
	Errors   []Error
	Renderer Renderer
}

func New(source string) *Arena {
	return &Arena{Source: source}
}

func (a *Arena) Add(err Error) {
	a.Errors = append(a.Errors, err)
}

func (a *Arena) AddDiagnostic(diagnostic Diagnostic) {
	if diagnostic == nil {
		return
	}
	a.Add(diagnostic.AsDiagnostic())
}

func (a *Arena) AddError(err error) bool {
	diagnostic, ok := FromError(err)
	if !ok {
		return false
	}
	a.Add(diagnostic)
	return true
}

func (a *Arena) HasDiagnostics() bool {
	return len(a.Errors) > 0
}

func (a *Arena) HasErrors() bool {
	return a.HasDiagnostics()
}

func (a *Arena) HasSeverity(severity Severity) bool {
	for _, err := range a.Errors {
		if err.Severity == severity {
			return true
		}
	}
	return false
}

func (a *Arena) HasFatalErrors() bool {
	return a.HasSeverity(SeverityError)
}

func (a *Arena) Clear() {
	a.Errors = nil
}

func (a *Arena) Error() string {
	if len(a.Errors) == 1 {
		return a.Errors[0].Error()
	}

	return fmt.Sprintf("diagnostics failed with %d errors", len(a.Errors))
}

func (a *Arena) Print(w io.Writer) {
	_ = a.Render(w)
}

func (a *Arena) PrintWith(w io.Writer, renderer Renderer) error {
	return a.render(w, renderer)
}

func (a *Arena) Render(w io.Writer) error {
	return a.render(w, a.Renderer)
}

func (a *Arena) RenderAuto(ctx context.Context, w io.Writer) error {
	return a.renderAuto(ctx, w, a.Renderer)
}

func (a *Arena) render(w io.Writer, renderer Renderer) error {
	errs := a.Errors
	a.Clear()

	if renderer == nil {
		renderer = DefaultRenderer()
	}

	for _, err := range errs {
		if renderErr := renderer.Render(w, a.Source, err.Localize()); renderErr != nil {
			return renderErr
		}
	}
	return nil
}

func (a *Arena) renderAuto(ctx context.Context, w io.Writer, renderer Renderer) error {
	errs := a.Errors
	a.Clear()

	if renderer == nil {
		renderer = DefaultRenderer()
	}

	for _, err := range errs {
		diagnostic, translateErr := err.LocalizeAuto(ctx)
		if translateErr != nil {
			return translateErr
		}
		if renderErr := renderer.Render(w, a.Source, diagnostic); renderErr != nil {
			return renderErr
		}
	}
	return nil
}

func (r *ColorRenderer) Render(w io.Writer, source string, err Error) error {
	if r == nil {
		r = DefaultRenderer()
	}

	var sb strings.Builder

	r.writeHeader(&sb, err)
	if err.IsShowSnippet {
		r.writeSnippet(&sb, source, err)
	}
	r.writeDescription(&sb, err)

	_, writeErr := fmt.Fprintln(w, sb.String())
	return writeErr
}

func (r *ColorRenderer) writeHeader(sb *strings.Builder, err Error) {
	view := r.severityView(err.Severity)
	codeName := err.CodeName
	if codeName == "" {
		codeName = view.Label
	}

	if view.Symbol != "" {
		sb.WriteString(r.apply(view.Symbol, view.SymbolStyles))
		sb.WriteString(" ")
	}
	sb.WriteString(r.apply(codeName, view.CodeStyles))
	sb.WriteString(r.apply(": ", r.Theme.MutedStyles))
	sb.WriteString(err.Message)
	sb.WriteString("\n")

	if err.Pos.Line != 0 {
		sb.WriteString(r.apply(r.Theme.LocationArrow, r.Theme.LocationStyles))
	} else {
		sb.WriteString(r.apply(r.Theme.ModuleLabel, r.Theme.LocationStyles))
	}
	sb.WriteString(r.apply(err.Pos.FileName, r.Theme.LocationStyles))
	sb.WriteString(" ")

	if err.Pos.Line != 0 {
		sb.WriteString(r.apply(fmt.Sprint(err.Pos.Line), r.Theme.LocationStyles))
		sb.WriteString(r.apply(":", r.Theme.LocationStyles))
		sb.WriteString(r.apply(fmt.Sprint(err.Pos.Column), r.Theme.LocationStyles))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("\n")
	}
}

func (r *ColorRenderer) writeSnippet(sb *strings.Builder, source string, err Error) {
	lines := getLines(source, int(err.Pos.Line), int(err.Pos.Line)-r.Theme.ContextLines)
	width := len(strconv.Itoa(strings.Count(source, "\n") + 1))

	for i, line := range lines {
		lineNum := int(err.Pos.Line) - len(lines) + i + 1

		sb.WriteString(r.apply(fmt.Sprintf("%*d │ ", width, lineNum), r.Theme.MutedStyles))
		sb.WriteString(line)
		sb.WriteString("\n")

		if lineNum == int(err.Pos.Line) {
			r.writeCaret(sb, line, err, width)
		}
	}
	sb.WriteString("\n")
}

func (r *ColorRenderer) writeCaret(sb *strings.Builder, line string, err Error, width int) {
	view := r.severityView(err.Severity)

	sb.WriteString(r.apply(fmt.Sprintf("%*s │ ", width, ""), r.Theme.MutedStyles))

	runes := []rune(line)
	col := clamp(int(err.Pos.Column)-1, 0, len(runes))
	sb.WriteString(strings.Repeat(" ", col))

	length := int(err.End - err.Start)
	if length <= 0 {
		length = 1
	}
	if col+length > len(runes) {
		length = len(runes) - col
	}
	if length <= 0 {
		length = 1
	}
	sb.WriteString(r.apply(strings.Repeat("^", length), view.CaretStyles))

	if err.Arrow != "" {
		sb.WriteString(" ")
		sb.WriteString(r.apply(err.Arrow, view.ArrowStyles))
	}
	sb.WriteString("\n")
}

func (r *ColorRenderer) writeDescription(sb *strings.Builder, err Error) {
	view := r.severityView(err.Severity)

	for _, desc := range err.Description {
		sb.WriteString(r.apply(r.Theme.DescriptionBullet, view.BulletStyles))
		sb.WriteString(r.apply(desc, view.DescriptionStyles))
		sb.WriteString("\n")
	}
}

func (r *ColorRenderer) severityView(severity Severity) SeverityView {
	if r == nil {
		return DefaultRenderTheme().SeverityViews[severity]
	}
	r.Theme = r.Theme.withDefaults()
	if view, ok := r.Theme.SeverityViews[severity]; ok {
		return view
	}
	return SeverityView{
		Symbol:            "?",
		Label:             severity.String(),
		SymbolStyles:      []colorista.Style{colorista.Bold, colorista.BrightWhite},
		CodeStyles:        []colorista.Style{colorista.Bold, colorista.BrightWhite},
		CaretStyles:       []colorista.Style{colorista.Bold, colorista.BrightWhite},
		ArrowStyles:       []colorista.Style{colorista.Bold, colorista.BrightWhite},
		BulletStyles:      []colorista.Style{colorista.Bold, colorista.BrightWhite},
		DescriptionStyles: []colorista.Style{},
	}
}

func (r *ColorRenderer) apply(text any, styles []colorista.Style) string {
	if r.Colorista == nil {
		r.Colorista = colorista.NewColorista(colorista.ThemeAuto)
	}
	return r.Colorista.Apply(text, styles...)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func getLines(source string, lineNumber, from int) []string {
	lines := strings.Split(source, "\n")
	idx := lineNumber - 1
	if idx < 0 || idx >= len(lines) {
		return nil
	}

	from = clamp(from, 0, idx)
	return lines[from : idx+1]
}

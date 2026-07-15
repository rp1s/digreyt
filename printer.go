// Package digerr
//
// Здесь допускается использование более тяжёлых по ресурсам решений,
// поскольку этот пакет вызывается только при возникновении ошибок.
package digerr

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rp1s/colorista"
)

const contextLines = 5

var color = diagnosticColors{colorista.NewColorista(colorista.ThemeAuto)}

type diagnosticColors struct {
	*colorista.Colorista
}

func (c diagnosticColors) BoldRed(text any) string {
	return c.Apply(text, colorista.Bold, colorista.BrightRed)
}

func (c diagnosticColors) Gray(text any) string {
	return c.Apply(text, colorista.BrightBlack)
}

func (c diagnosticColors) Blue(text any) string {
	return c.Apply(text, colorista.BrightBlue)
}

func (c diagnosticColors) BoldBlue(text any) string {
	return c.Apply(text, colorista.Bold, colorista.BrightBlue)
}

func (c diagnosticColors) Black(text any) string {
	return c.Apply(text, colorista.BrightBlack)
}

func (c diagnosticColors) PastelYellow(text any) string {
	return c.Apply(text, colorista.Rgb(colorista.RGB{R: 255, G: 221, B: 120}))
}

func (c diagnosticColors) BoldGreen(text any) string {
	return c.Apply(text, colorista.Bold, colorista.BrightGreen)
}

type Arena struct {
	Source string
	Errors []Error
}

func New(source string) *Arena {
	return &Arena{Source: source}
}

func (a *Arena) Add(err Error) {
	a.Errors = append(a.Errors, err)
}

func (a *Arena) HasErrors() bool {
	return len(a.Errors) > 0
}

func (a *Arena) HasFatalErrors() bool {
	for _, err := range a.Errors {
		if err.Severity == SeverityError {
			return true
		}
	}
	return false
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
	errs := a.Errors
	a.Clear()

	for _, err := range errs {
		writeError(w, a.Source, err)
	}
}

func writeError(w io.Writer, source string, err Error) {
	var sb strings.Builder

	writeHeader(&sb, err)
	if err.IsShowSnippet {
		writeSnippet(&sb, source, err)
	}
	writeDescription(&sb, err)

	fmt.Fprintln(w, sb.String())
}

func writeHeader(sb *strings.Builder, err Error) {
	sb.WriteString(color.BoldRed("× "))
	sb.WriteString(color.BoldRed(err.CodeName))
	sb.WriteString(color.Gray(": "))
	sb.WriteString(err.Message)
	sb.WriteString("\n")

	if err.Pos.Line != 0 {
		sb.WriteString(color.Blue("──> "))
	} else {
		sb.WriteString(color.Blue("Module: "))
	}
	sb.WriteString(color.BoldBlue(err.Pos.FileName))
	sb.WriteString(" ")

	if err.Pos.Line != 0 {
		sb.WriteString(color.BoldBlue(fmt.Sprint(err.Pos.Line)))
		sb.WriteString(color.BoldBlue(":"))
		sb.WriteString(color.BoldBlue(fmt.Sprint(err.Pos.Column)))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("\n")
	}
}

func writeSnippet(sb *strings.Builder, source string, err Error) {
	lines := getLines(source, int(err.Pos.Line), int(err.Pos.Line)-contextLines)
	width := len(strconv.Itoa(strings.Count(source, "\n") + 1))

	for i, line := range lines {
		lineNum := int(err.Pos.Line) - len(lines) + i + 1

		sb.WriteString(color.Black(fmt.Sprintf("%*d │ ", width, lineNum)))
		sb.WriteString(line)
		sb.WriteString("\n")

		if lineNum == int(err.Pos.Line) {
			writeCaret(sb, line, err, width)
		}
	}
	sb.WriteString("\n")
}

func writeCaret(sb *strings.Builder, line string, err Error, width int) {
	sb.WriteString(color.Black(fmt.Sprintf("%*s │ ", width, "")))

	runes := []rune(line)
	col := clamp(int(err.Pos.Column)-1, 0, len(runes))
	sb.WriteString(strings.Repeat(" ", col))

	length := int(err.End - err.Start)
	if length <= 0 {
		length = 1
	}
	sb.WriteString(color.BoldRed(strings.Repeat("^", length)))

	sb.WriteString(" ")
	sb.WriteString(color.PastelYellow(err.Arrow))
	sb.WriteString("\n")
}

func writeDescription(sb *strings.Builder, err Error) {
	for _, desc := range err.Description {
		sb.WriteString(color.BoldGreen("• "))
		sb.WriteString(desc)
		sb.WriteString("\n")
	}
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

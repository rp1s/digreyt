package digerr

import "github.com/CandyCrafts/candy/pkg/digreyt/translate"

type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "error"
	}
}

type Error struct {
	Code          uint16
	Severity      Severity
	CodeName      string
	Message       string
	Arrow         string
	IsShowSnippet bool
	Description   []string

	MessageTranslations     translate.Translations
	ArrowTranslations       translate.Translations
	DescriptionTranslations []translate.Translations

	Start uint64
	End   uint64
	Pos   Position
}

type Span struct {
	Start uint64
	End   uint64
	Pos   Position
}

type Position struct {
	FileName string
	Line     uint64
	Column   uint64
	Offset   uint64
}

func (e Error) Update(span Span) Error {
	e.Start = span.Start
	e.End = span.End
	e.Pos = span.Pos
	return e.Localize()
}

func (e Error) IU(fileModule string, description []string) Error {
	e = e.Localize()
	e.Description = description
	e.Pos.FileName = fileModule
	e.Pos.Line = 0
	return e
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Localize().Message
}

func (e Error) Localize() Error {
	if len(e.MessageTranslations) > 0 {
		e.Message = translate.Resolve(e.MessageTranslations)
	}
	if len(e.ArrowTranslations) > 0 {
		e.Arrow = translate.Resolve(e.ArrowTranslations)
	}
	if len(e.DescriptionTranslations) > 0 {
		e.Description = make([]string, 0, len(e.DescriptionTranslations))
		for _, desc := range e.DescriptionTranslations {
			e.Description = append(e.Description, translate.Resolve(desc))
		}
	}
	return e
}

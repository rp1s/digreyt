package digreyt

import (
	"context"
	stderrors "errors"

	"github.com/rp1s/digreyt/translate"
)

type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeveritySuccess
)

func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeveritySuccess:
		return "success"
	default:
		return "error"
	}
}

type Diagnostic interface {
	AsDiagnostic() Error
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

func FromError(err error) (Error, bool) {
	if err == nil {
		return Error{}, false
	}

	var diagnostic Diagnostic
	if stderrors.As(err, &diagnostic) {
		return diagnostic.AsDiagnostic(), true
	}

	return Error{
		Severity: SeverityError,
		CodeName: "Error",
		Message:  err.Error(),
	}, true
}

func (e Error) AsDiagnostic() Error {
	return e.Localize()
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

func (e Error) LocalizeAuto(ctx context.Context) (Error, error) {
	var err error
	if len(e.MessageTranslations) > 0 {
		e.Message, err = translate.ResolveAuto(ctx, e.MessageTranslations)
		if err != nil {
			return e, err
		}
	}
	if len(e.ArrowTranslations) > 0 {
		e.Arrow, err = translate.ResolveAuto(ctx, e.ArrowTranslations)
		if err != nil {
			return e, err
		}
	}
	if len(e.DescriptionTranslations) > 0 {
		e.Description = make([]string, 0, len(e.DescriptionTranslations))
		for _, desc := range e.DescriptionTranslations {
			text, err := translate.ResolveAuto(ctx, desc)
			if err != nil {
				return e, err
			}
			e.Description = append(e.Description, text)
		}
	}
	return e, nil
}

# digreyt

`digreyt` is a Go diagnostic renderer for compilers, parsers, linters and CLI
tools. It can render errors, warnings, info messages and success diagnostics.

The default terminal renderer is built on top of
[`github.com/rp1s/colorista`](https://github.com/rp1s/colorista). You can keep
the default layout, customize only colors/symbols, or replace the whole renderer
with your own output format.

## Install

```sh
go get github.com/rp1s/digreyt
```

## Basic Error

```go
arena := digreyt.New(source)
arena.Add(digreyt.Error{
	CodeName:      "ParseError",
	Message:       "unexpected token",
	Arrow:         "expected expression",
	Severity:      digreyt.SeverityError,
	IsShowSnippet: true,

package digerr

import (
	"os"
	"testing"
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

	if !arena.HasErrors() {
		t.Errorf("Expected arena to have errors, but it does not.")
	}

	arena.Print(os.Stderr)

	arena.Clear()

	if arena.HasErrors() {
		t.Errorf("Expected arena to be cleared of errors, but it still has errors.")
	}
}

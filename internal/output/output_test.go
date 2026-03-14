package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintCSV(t *testing.T) {
	var buf bytes.Buffer
	err := Print(&buf, FormatCSV, []string{"name", "value"}, []map[string]any{{"name": "temp", "value": "72.0F"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "temp,72.0F") {
		t.Fatalf("unexpected csv output %q", buf.String())
	}
}

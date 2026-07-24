package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shadowfish07/heybox-cli/internal/search"
)

func TestJSONUsesStableEnvelope(t *testing.T) {
	t.Parallel()
	var buffer bytes.Buffer
	page := search.Page{Query: "steam", Type: "all", Page: 1, Limit: 20, Results: []search.Result{{Type: "topic", ID: "1", Title: "中文话题"}}}
	if err := JSON(&buffer, page); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"query": "steam"`, `"type": "topic"`, `"title": "中文话题"`} {
		if !strings.Contains(buffer.String(), expected) {
			t.Fatalf("JSON output missing %s:\n%s", expected, buffer.String())
		}
	}
}

func TestTableContainsUnicodeAndURL(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	var buffer bytes.Buffer
	page := search.Page{Results: []search.Result{{Type: "post", Title: "中文标题", Summary: "一段摘要", URL: "https://example.com/1"}}}
	if err := Table(&buffer, page); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "中文标题") || !strings.Contains(buffer.String(), "https://example.com/1") {
		t.Fatalf("unexpected table output:\n%s", buffer.String())
	}
}

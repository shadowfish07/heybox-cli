package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"search", "--help"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "--max-pages") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestValidationReturnsUsageExitCode(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"search", "steam", "--type", "invalid"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "无效的 --type") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

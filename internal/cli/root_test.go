package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		want     string
	}{
		{name: "release injection wins", injected: "1.2.3", info: &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, want: "1.2.3"},
		{name: "go install module version", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, want: "1.2.3"},
		{name: "local build", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveVersion(test.injected, test.info); got != test.want {
				t.Fatalf("resolveVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

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

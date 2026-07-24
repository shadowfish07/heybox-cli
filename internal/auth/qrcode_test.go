package auth

import (
	"bytes"
	"os"
	"runtime"
	"testing"
)

func TestQRCodeOutputs(t *testing.T) {
	t.Parallel()
	const content = "https://example.com/login?qr=test"
	var terminal bytes.Buffer
	if err := WriteTerminalQRCode(&terminal, content); err != nil {
		t.Fatal(err)
	}
	if terminal.Len() == 0 {
		t.Fatal("terminal QR code is empty")
	}
	path, cleanup, err := CreateTemporaryQRCodePNG(content)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("PNG is empty")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("PNG mode = %04o", info.Mode().Perm())
	}
}

package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCredentialCookie(t *testing.T) {
	t.Parallel()
	credential := Credential{HeyboxID: "123456", PKey: "secret-pkey"}
	cookie := credential.Cookie()
	for _, expected := range []string{
		"heybox_id=123456",
		"user_heybox_id=123456",
		"pkey=secret-pkey",
		"user_pkey=secret-pkey",
		"x_xhh_heyboxid=123456",
	} {
		if !strings.Contains(cookie, expected) {
			t.Fatalf("Cookie() = %q, missing %q", cookie, expected)
		}
	}
}

func TestFileStoreRoundTripAndPermissions(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "config", "heybox-cli")
	path := filepath.Join(directory, "session.json")
	store := NewFileStoreAt(path)
	want := Credential{HeyboxID: "123456", PKey: "secret", ExpireAt: "1999999999"}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		directoryInfo, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if got := directoryInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("directory mode = %04o", got)
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fileInfo.Mode().Perm(); got != 0o600 {
			t.Fatalf("file mode = %04o", got)
		}
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
	if err := store.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestFileStoreRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated permissions")
	}
	t.Parallel()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target.json")
	if err := os.WriteFile(target, []byte(`{"heybox_id":"1","pkey":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "session.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	store := NewFileStoreAt(link)
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.Save(Credential{HeyboxID: "2", PKey: "new"}); err == nil || !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestFileStoreRejectsSymlinkDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated permissions")
	}
	t.Parallel()
	root := t.TempDir()
	realDirectory := filepath.Join(root, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedDirectory := filepath.Join(root, "linked")
	if err := os.Symlink(realDirectory, linkedDirectory); err != nil {
		t.Fatal(err)
	}
	store := NewFileStoreAt(filepath.Join(linkedDirectory, "session.json"))
	if err := store.Save(Credential{HeyboxID: "2", PKey: "new"}); err == nil || !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("Save() error = %v", err)
	}
}

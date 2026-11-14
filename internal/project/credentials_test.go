package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const secret = "sk-test-SECRET-9876"

func credDir(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("CERNE_CONFIG_DIR", dir)
	return dir
}

func TestCredentialWriteReadPermissions(t *testing.T) {
	dir := credDir(t)
	if err := WriteCredential("MY_KEY", secret); err != nil {
		t.Fatal(err)
	}
	if err := WriteCredential("OTHER", "x"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials.env")
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Fatalf("file mode = %v, want 0600", fi.Mode().Perm())
		}
	}
	b, _ := os.ReadFile(path)
	if string(b) != "MY_KEY="+secret+"\nOTHER=x\n" {
		t.Fatalf("not .env format: %q", b)
	}
	v, src, err := ResolveAPIKey("MY_KEY")
	if v != secret || src != "file" || err != nil {
		t.Fatalf("got %q %q %v", v, src, err)
	}
	if err := DeleteCredential("MY_KEY"); err != nil {
		t.Fatal(err)
	}
	if v, src, _ := ResolveAPIKey("MY_KEY"); v != "" || src != "" {
		t.Fatalf("still set after delete: %q %q", v, src)
	}
	if v, _, _ := ResolveAPIKey("OTHER"); v != "x" {
		t.Fatal("delete removed a sibling")
	}
}

func TestCredentialEnvWinsOverFile(t *testing.T) {
	credDir(t)
	if err := WriteCredential("MY_KEY", "from-file"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MY_KEY", "from-env")
	if v, src, _ := ResolveAPIKey("MY_KEY"); v != "from-env" || src != "env" {
		t.Fatalf("got %q %q, want the env var to win", v, src)
	}
}

func TestCredentialOpenPermissionRefusedWithoutLeaking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no permission bits on windows")
	}
	dir := credDir(t)
	path := filepath.Join(dir, "credentials.env")
	if err := os.WriteFile(path, []byte("MY_KEY="+secret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := ResolveAPIKey("MY_KEY")
	if !errors.Is(err, ErrCredentialsInsecure) {
		t.Fatalf("err = %v, want ErrCredentialsInsecure", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("the key value leaked into the error")
	}
}

func TestCredentialRejectsBadInput(t *testing.T) {
	credDir(t)
	if err := WriteCredential("bad name", "v"); !errors.Is(err, ErrCredentialName) {
		t.Errorf("name: %v", err)
	}
	for _, v := range []string{"", "a\nB=c"} {
		if err := WriteCredential("K", v); !errors.Is(err, ErrCredentialValue) {
			t.Errorf("value %q: %v", v, err)
		}
	}
}

func TestCredentialParsesCommentsAndQuotes(t *testing.T) {
	dir := credDir(t)
	os.WriteFile(filepath.Join(dir, "credentials.env"), []byte("# c\n\nA=\"quoted\"\nB = spaced \n"), 0o600)
	if v, _, _ := ResolveAPIKey("A"); v != "quoted" {
		t.Errorf("A = %q", v)
	}
	if v, _, _ := ResolveAPIKey("B"); v != "spaced" {
		t.Errorf("B = %q", v)
	}
}

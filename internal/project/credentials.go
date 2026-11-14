package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// credentialsFile is the host-only secrets file under ConfigDir (F-0414): .env
// format, one NAME=value per line, never in a project and never in git. An
// environment variable of the same NAME always wins over it.
const credentialsFile = "credentials.env"

// ErrCredentialsInsecure: the credentials file is readable by group or others,
// so it is refused rather than trusted (the key may already have leaked).
var ErrCredentialsInsecure = errors.New("credentials file is readable by other users")

// ErrCredentialName / ErrCredentialValue: the NAME is not a shell-style
// variable name, or the value cannot live on one .env line.
var (
	ErrCredentialName  = errors.New("invalid credential name")
	ErrCredentialValue = errors.New("invalid credential value")
)

var credentialNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func credentialsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credentialsFile), nil
}

// readCredentials parses the file; a missing one is an empty set. A file with
// group/other permission bits is refused (unix only: Windows has no such bits).
// Errors never carry file content.
func readCredentials() (map[string]string, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%w: chmod 600 %s", ErrCredentialsInsecure, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		name, val, ok := strings.Cut(line, "=")
		if line == "" || line[0] == '#' || !ok {
			continue
		}
		val = strings.TrimSpace(val)
		if n := len(val); n >= 2 && (val[0] == '"' || val[0] == '\'') && val[n-1] == val[0] {
			val = val[1 : n-1]
		}
		out[strings.TrimSpace(name)] = val
	}
	return out, nil
}

// ResolveAPIKey returns the key stored under NAME and where it came from:
// "env" (the process environment, which wins) or "file" (the host credentials
// file). Empty value = no key anywhere. The value is for the caller to use,
// never to log or return over HTTP.
func ResolveAPIKey(name string) (value, source string, err error) {
	if v := os.Getenv(name); v != "" {
		return v, "env", nil
	}
	creds, err := readCredentials()
	if err != nil {
		return "", "", err
	}
	if v := creds[name]; v != "" {
		return v, "file", nil
	}
	return "", "", nil
}

// WriteCredential stores NAME=value in the host credentials file (directory
// 0700, file 0600, replaced atomically through a temp file + rename).
func WriteCredential(name, value string) error {
	if !credentialNameRE.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrCredentialName, name)
	}
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return ErrCredentialValue
	}
	creds, err := readCredentials()
	if err != nil {
		return err
	}
	creds[name] = value
	return saveCredentials(creds)
}

// DeleteCredential removes NAME from the file; absent is not an error.
func DeleteCredential(name string) error {
	creds, err := readCredentials()
	if err != nil {
		return err
	}
	if _, ok := creds[name]; !ok {
		return nil
	}
	delete(creds, name)
	return saveCredentials(creds)
}

func saveCredentials(creds map[string]string) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var names []string
	for n := range creds {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		sb.WriteString(n + "=" + creds[n] + "\n")
	}
	tmp, err := os.CreateTemp(dir, credentialsFile+".*.tmp")
	if err != nil {
		return err
	}
	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := tmp.WriteString(sb.String()); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func desktopValue(t *testing.T, key string) string {
	t.Helper()
	for _, line := range strings.Split(readRepoFile(t, "packaging/robsong.desktop"), "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func TestDesktopExecAcceptsFiles(t *testing.T) {
	if got := desktopValue(t, "Exec"); got != "robsong %F" {
		t.Fatalf("Exec = %q, want %q", got, "robsong %F")
	}
}

func TestDesktopMimeTypes(t *testing.T) {
	got := map[string]bool{}
	for _, m := range strings.Split(desktopValue(t, "MimeType"), ";") {
		if m != "" {
			got[m] = true
		}
	}
	for _, want := range []string{
		"audio/mpeg", "audio/mp3", "audio/flac", "audio/x-flac",
		"audio/wav", "audio/x-wav", "audio/ogg", "audio/x-vorbis+ogg",
	} {
		if !got[want] {
			t.Errorf("MimeType missing %s", want)
		}
	}
}

func TestDesktopFileValidates(t *testing.T) {
	bin, err := exec.LookPath("desktop-file-validate")
	if err != nil {
		t.Skip("desktop-file-validate not on PATH")
	}
	out, err := exec.Command(bin, filepath.Join("..", "..", "packaging", "robsong.desktop")).CombinedOutput()
	if err != nil {
		t.Fatalf("desktop-file-validate: %v\n%s", err, out)
	}
}

func TestNfpmScriptsRefreshDesktopDatabase(t *testing.T) {
	cfg := readRepoFile(t, "nfpm.yaml")
	for _, key := range []string{"postinstall", "postremove"} {
		m := regexp.MustCompile(`(?m)^\s*` + key + `:\s*(\S+)\s*$`).FindStringSubmatch(cfg)
		if m == nil {
			t.Errorf("nfpm.yaml has no %s script", key)
			continue
		}
		script, err := os.ReadFile(filepath.Join("..", "..", m[1]))
		if err != nil {
			t.Errorf("%s script %s: %v", key, m[1], err)
			continue
		}
		s := string(script)
		if !strings.Contains(s, "update-desktop-database") {
			t.Errorf("%s does not call update-desktop-database", m[1])
		}
		if !strings.Contains(s, "|| true") {
			t.Errorf("%s must tolerate failure with '|| true'", m[1])
		}
	}
}

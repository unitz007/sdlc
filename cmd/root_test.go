package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkDir(t *testing.T) {
	t.Run("empty string returns current working directory", func(t *testing.T) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd() failed: %v", err)
		}
		got, err := resolveWorkDir("")
		if err != nil {
			t.Fatalf("resolveWorkDir(\"\") returned error: %v", err)
		}
		if got != cwd {
			t.Errorf("resolveWorkDir(\"\") = %q, want %q", got, cwd)
		}
	})

	t.Run("absolute path is returned unchanged", func(t *testing.T) {
		tmpDir := t.TempDir()
		got, err := resolveWorkDir(tmpDir)
		if err != nil {
			t.Fatalf("resolveWorkDir(%q) returned error: %v", tmpDir, err)
		}
		if got != tmpDir {
			t.Errorf("resolveWorkDir(%q) = %q, want %q", tmpDir, got, tmpDir)
		}
	})

	t.Run("tilde prefix is expanded to home directory", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir() failed: %v", err)
		}
		got, err := resolveWorkDir("~/Documents")
		if err != nil {
			t.Fatalf("resolveWorkDir(\"~/Documents\") returned error: %v", err)
		}
		expected := filepath.Join(homeDir, "Documents")
		if got != expected {
			t.Errorf("resolveWorkDir(\"~/Documents\") = %q, want %q", got, expected)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expanded path %q is not absolute", got)
		}
	})

	t.Run("relative path returns error", func(t *testing.T) {
		_, err := resolveWorkDir("subdir")
		if err == nil {
			t.Fatal("resolveWorkDir(\"subdir\") expected error, got nil")
		}
		if !strings.Contains(err.Error(), "absolute path") {
			t.Errorf("error %q should contain 'absolute path'", err.Error())
		}
	})

	t.Run("dot-slash relative path returns error", func(t *testing.T) {
		_, err := resolveWorkDir("./relative")
		if err == nil {
			t.Fatal("resolveWorkDir(\"./relative\") expected error, got nil")
		}
		if !strings.Contains(err.Error(), "absolute path") {
			t.Errorf("error %q should contain 'absolute path'", err.Error())
		}
	})

	t.Run("dot-dot-slash relative path returns error", func(t *testing.T) {
		_, err := resolveWorkDir("../relative")
		if err == nil {
			t.Fatal("resolveWorkDir(\"../relative\") expected error, got nil")
		}
		if !strings.Contains(err.Error(), "absolute path") {
			t.Errorf("error %q should contain 'absolute path'", err.Error())
		}
	})
}

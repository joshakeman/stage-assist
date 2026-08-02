package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "does-not-exist.env")); err != nil {
		t.Fatalf("loadDotEnv on a missing file returned an error: %v", err)
	}
}

func TestLoadDotEnvSetsValuesSkippingBlanksCommentsAndMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	contents := "FOO=bar\n# a comment\n\nMALFORMED_LINE_NO_EQUALS\nBAZ=qux\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	os.Unsetenv("FOO")
	os.Unsetenv("BAZ")
	os.Unsetenv("MALFORMED_LINE_NO_EQUALS")
	t.Cleanup(func() {
		os.Unsetenv("FOO")
		os.Unsetenv("BAZ")
	})

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}

	if got := os.Getenv("FOO"); got != "bar" {
		t.Errorf(`FOO = %q, want "bar"`, got)
	}
	if got := os.Getenv("BAZ"); got != "qux" {
		t.Errorf(`BAZ = %q, want "qux"`, got)
	}
	if _, set := os.LookupEnv("MALFORMED_LINE_NO_EQUALS"); set {
		t.Errorf("MALFORMED_LINE_NO_EQUALS should not have been set from a line with no '='")
	}
}

func TestLoadDotEnvNeverOverridesAnExistingRealEnvVar(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("EXISTING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	os.Setenv("EXISTING", "from-shell")
	t.Cleanup(func() { os.Unsetenv("EXISTING") })

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}

	if got := os.Getenv("EXISTING"); got != "from-shell" {
		t.Errorf("EXISTING = %q, want the real environment's value (\"from-shell\") to win over the .env file", got)
	}
}

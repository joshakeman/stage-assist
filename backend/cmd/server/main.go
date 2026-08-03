package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/api"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("loading .env: %v", err)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required (set it in backend/.env, see .env.example)")
	}
	interpreter := aiparse.NewAnthropicInterpreter(apiKey)

	mux := api.NewMux(interpreter)
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// loadDotEnv reads simple KEY=VALUE lines from path, if it exists, and sets
// them as environment variables -- but never overrides a variable already
// present in the real environment. Deliberately minimal (no quoting, no
// multiline values) rather than a dependency: local dev needs nothing more
// than this to keep secrets like ANTHROPIC_API_KEY out of the shell profile
// and out of git (see .gitignore).
func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		os.Setenv(key, strings.TrimSpace(value))
	}
	return nil
}

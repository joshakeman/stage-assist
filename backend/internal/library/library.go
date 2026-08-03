// Package library persists a user's confirmed, already-reviewed scripts
// locally so they can be reloaded later without re-uploading a PDF or
// paying for another AI interpretation of it -- see CLAUDE.md's "Saved
// script library" section for why this exists and what it doesn't change.
//
// This package has no notion of PDFs, AI, or grounding; it only stores
// and retrieves the same minimal {kind, character, text} shape the rest
// of the app already treats as "a script's confirmed content" once a
// human has reviewed it.
package library

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound means no saved script exists with the given ID.
var ErrNotFound = errors.New("library: saved script not found")

// Element is one line of a saved script's confirmed content -- the same
// {kind, character, text} shape as the API layer's confirmedElementDTO,
// but this package's own type so internal/library has no dependency on
// internal/api (api depends on library, never the reverse).
type Element struct {
	Kind      string
	Character string
	Text      string
}

// SavedScript is one named, confirmed script a user has chosen to keep.
type SavedScript struct {
	ID        string
	Name      string
	CreatedAt time.Time
	Elements  []Element
}

// Store persists SavedScripts in a local SQLite file. There is no
// interface here: a real *Store is local, fast, and deterministic --
// exactly the case CLAUDE.md's "Avoid speculative abstractions" section
// says does NOT need one (contrast aiparse.ScriptInterpreter, which does,
// because a real implementation there means a real network call with
// cost, latency, and non-determinism a fake is worth having). Tests use a
// real *Store, typically against SQLite's ":memory:" DSN.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS saved_scripts (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	created_at    TEXT NOT NULL,
	elements_json TEXT NOT NULL
);
`

// NewStore opens (creating if necessary) a SQLite database at path and
// ensures its schema exists. path may be ":memory:" for a private,
// in-process database, which is what tests use instead of a temp file.
func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("library: opening database: %w", err)
	}

	// WAL reduces writer/reader contention under Go's concurrent
	// http.Server -- a real concern once this is served over HTTP, not a
	// speculative one. *sql.DB itself is already safe for concurrent use
	// across goroutines (it manages its own connection pool), so no
	// additional locking is needed here.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("library: setting journal mode: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("library: creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// List returns every saved script, newest first.
func (s *Store) List() ([]SavedScript, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at, elements_json FROM saved_scripts ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("library: listing saved scripts: %w", err)
	}
	defer rows.Close()

	var scripts []SavedScript
	for rows.Next() {
		script, err := scanSavedScript(rows)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, script)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("library: listing saved scripts: %w", err)
	}
	return scripts, nil
}

// Get returns the saved script with id, or ErrNotFound if none exists.
func (s *Store) Get(id string) (SavedScript, error) {
	row := s.db.QueryRow(`SELECT id, name, created_at, elements_json FROM saved_scripts WHERE id = ?`, id)
	script, err := scanSavedScript(row)
	if errors.Is(err, sql.ErrNoRows) {
		return SavedScript{}, ErrNotFound
	}
	if err != nil {
		return SavedScript{}, fmt.Errorf("library: getting saved script: %w", err)
	}
	return script, nil
}

// Save creates a new saved script with name and elements, generating its
// ID and CreatedAt. It always creates a new record -- there is no
// rename/update-in-place in this package.
func (s *Store) Save(name string, elements []Element) (SavedScript, error) {
	id, err := newID()
	if err != nil {
		return SavedScript{}, fmt.Errorf("library: generating id: %w", err)
	}

	elementsJSON, err := json.Marshal(elements)
	if err != nil {
		return SavedScript{}, fmt.Errorf("library: encoding elements: %w", err)
	}

	script := SavedScript{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		Elements:  elements,
	}

	_, err = s.db.Exec(
		`INSERT INTO saved_scripts (id, name, created_at, elements_json) VALUES (?, ?, ?, ?)`,
		script.ID, script.Name, script.CreatedAt.Format(time.RFC3339Nano), elementsJSON,
	)
	if err != nil {
		return SavedScript{}, fmt.Errorf("library: saving script: %w", err)
	}
	return script, nil
}

// Delete removes the saved script with id, or returns ErrNotFound if none
// exists.
func (s *Store) Delete(id string) error {
	result, err := s.db.Exec(`DELETE FROM saved_scripts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("library: deleting saved script: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("library: deleting saved script: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting
// scanSavedScript serve Get and List alike.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSavedScript(row rowScanner) (SavedScript, error) {
	var (
		script       SavedScript
		createdAt    string
		elementsJSON string
	)
	if err := row.Scan(&script.ID, &script.Name, &createdAt, &elementsJSON); err != nil {
		return SavedScript{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return SavedScript{}, fmt.Errorf("library: parsing created_at: %w", err)
	}
	script.CreatedAt = parsedCreatedAt

	if err := json.Unmarshal([]byte(elementsJSON), &script.Elements); err != nil {
		return SavedScript{}, fmt.Errorf("library: parsing elements: %w", err)
	}
	return script, nil
}

// newID generates a random, opaque identifier -- not derived from the
// user-supplied name (arbitrary text is a poor key) or from time
// (CreatedAt already gives sortability).
func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

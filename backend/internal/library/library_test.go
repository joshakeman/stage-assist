package library_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/library"
)

// newTestStore uses SQLite's shared-cache in-memory mode, not a bare
// ":memory:" DSN. A bare ":memory:" gives every new connection its own
// private, empty database -- harmless for a test that only ever uses one
// connection, but under concurrent access Go's connection pool opens more
// than one, and each additional connection sees a database that never
// ran the schema. Shared-cache mode, keyed by the test's own name so
// tests never collide, makes every connection within one *Store see the
// same in-memory database.
func newTestStore(t *testing.T) *library.Store {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	store, err := library.NewStore(dsn)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSaveAndGetRoundTrip(t *testing.T) {
	store := newTestStore(t)

	elements := []library.Element{
		{Kind: "dialogue", Character: "HAMLET", Text: "Who's there?"},
		{Kind: "direction", Character: "", Text: "Enter FRANCISCO."},
	}
	saved, err := store.Save("Hamlet, Act 1", elements)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("Save did not assign an ID")
	}
	if saved.Name != "Hamlet, Act 1" {
		t.Errorf("Name = %q, want %q", saved.Name, "Hamlet, Act 1")
	}

	got, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != saved.Name || len(got.Elements) != 2 {
		t.Errorf("Get returned %+v, want a match for %+v", got, saved)
	}
	if got.Elements[0] != elements[0] || got.Elements[1] != elements[1] {
		t.Errorf("Elements = %+v, want %+v", got.Elements, elements)
	}
	if !got.CreatedAt.Equal(saved.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, saved.CreatedAt)
	}
}

func TestListOrdersNewestFirst(t *testing.T) {
	store := newTestStore(t)

	first, err := store.Save("First", []library.Element{{Kind: "dialogue", Character: "A", Text: "one"}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	second, err := store.Save("Second", []library.Element{{Kind: "dialogue", Character: "A", Text: "two"}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d scripts, want 2", len(got))
	}
	if got[0].ID != second.ID || got[1].ID != first.ID {
		t.Errorf("List order = [%s, %s], want newest first: [%s, %s]", got[0].Name, got[1].Name, second.Name, first.Name)
	}
}

func TestDeleteRemovesTheRecord(t *testing.T) {
	store := newTestStore(t)

	saved, err := store.Save("To delete", []library.Element{{Kind: "dialogue", Character: "A", Text: "line"}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Delete(saved.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Get(saved.ID); !errors.Is(err, library.ErrNotFound) {
		t.Errorf("Get after Delete: err = %v, want ErrNotFound", err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("List after Delete = %d scripts, want 0", len(all))
	}
}

func TestGetAndDeleteReturnErrNotFoundForUnknownID(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Get("does-not-exist"); !errors.Is(err, library.ErrNotFound) {
		t.Errorf("Get: err = %v, want ErrNotFound", err)
	}
	if err := store.Delete("does-not-exist"); !errors.Is(err, library.ErrNotFound) {
		t.Errorf("Delete: err = %v, want ErrNotFound", err)
	}
}

func TestEmptyStoreListsAsEmptyNotError(t *testing.T) {
	store := newTestStore(t)

	got, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d scripts, want 0", len(got))
	}
}

// TestConcurrentSavesAllPersist is the real test of the "*sql.DB is safe
// for concurrent goroutine use" claim this package relies on instead of
// adding its own mutex -- not boilerplate.
func TestConcurrentSavesAllPersist(t *testing.T) {
	store := newTestStore(t)

	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Save("concurrent", []library.Element{{Kind: "dialogue", Character: "A", Text: "line"}})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Save #%d: %v", i, err)
		}
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != n {
		t.Errorf("got %d saved scripts, want %d (one per concurrent Save)", len(all), n)
	}
}

// TestPersistsAcrossStoreInstances is the unit-level version of "survives
// a server restart": a second *Store opened against the same file must
// see what a first *Store wrote.
func TestPersistsAcrossStoreInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scripts.db")

	first, err := library.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	saved, err := first.Save("Persisted", []library.Element{{Kind: "dialogue", Character: "A", Text: "line"}})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := library.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (second instance): %v", err)
	}
	defer second.Close()

	got, err := second.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get from second instance: %v", err)
	}
	if got.Name != "Persisted" {
		t.Errorf("Name = %q, want %q", got.Name, "Persisted")
	}
}

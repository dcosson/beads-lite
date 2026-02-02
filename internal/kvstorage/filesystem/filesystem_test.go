package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"beads-lite/internal/kvstorage"
)

func newTestStore(t *testing.T, table string) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, table)
	if err != nil {
		t.Fatalf("New(%q): %v", table, err)
	}
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func TestNew_ReservedTable(t *testing.T) {
	for _, name := range kvstorage.ReservedTableNames {
		_, err := New(t.TempDir(), name)
		if err == nil {
			t.Errorf("New(%q) should fail for reserved table name", name)
		}
		if !errors.Is(err, kvstorage.ErrReservedTable) {
			t.Errorf("New(%q) error = %v, want ErrReservedTable", name, err)
		}
	}
}

func TestNew_EmptyTable(t *testing.T) {
	_, err := New(t.TempDir(), "")
	if err == nil {
		t.Fatal("New with empty table name should fail")
	}
}

func TestNew_ValidTable(t *testing.T) {
	s, err := New(t.TempDir(), "slots")
	if err != nil {
		t.Fatalf("New(slots): %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSetAndGet(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	data := []byte(`{"name":"alice"}`)
	if err := s.Set(ctx, "key1", data, kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Get = %q, want %q", got, data)
	}
}

func TestSet_Overwrite(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	if err := s.Set(ctx, "k", []byte("v1"), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set v1: %v", err)
	}
	if err := s.Set(ctx, "k", []byte("v2"), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set v2: %v", err)
	}

	got, err := s.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("Get = %q, want %q", got, "v2")
	}
}

func TestSet_FailIfExists(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	if err := s.Set(ctx, "k", []byte("v1"), kvstorage.SetOptions{Exists: kvstorage.FailIfExists}); err != nil {
		t.Fatalf("Set first: %v", err)
	}

	err := s.Set(ctx, "k", []byte("v2"), kvstorage.SetOptions{Exists: kvstorage.FailIfExists})
	if err == nil {
		t.Fatal("Set with FailIfExists should fail for existing key")
	}
	if !errors.Is(err, kvstorage.ErrAlreadyExists) {
		t.Errorf("error = %v, want ErrAlreadyExists", err)
	}

	// Original value should be preserved
	got, _ := s.Get(ctx, "k")
	if string(got) != "v1" {
		t.Errorf("Get = %q, want %q (original)", got, "v1")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Get nonexistent should fail")
	}
	if !errors.Is(err, kvstorage.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestSet_FailIfNotExists(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	if err := s.Set(ctx, "k", []byte("v1"), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set(ctx, "k", []byte("v2"), kvstorage.SetOptions{Exists: kvstorage.FailIfNotExists}); err != nil {
		t.Fatalf("Set with FailIfNotExists: %v", err)
	}

	got, _ := s.Get(ctx, "k")
	if string(got) != "v2" {
		t.Errorf("Get = %q, want %q", got, "v2")
	}
}

func TestSet_FailIfNotExists_NotFound(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	err := s.Set(ctx, "nonexistent", []byte("v"), kvstorage.SetOptions{Exists: kvstorage.FailIfNotExists})
	if err == nil {
		t.Fatal("Set with FailIfNotExists on nonexistent key should fail")
	}
	if !errors.Is(err, kvstorage.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	if err := s.Set(ctx, "k", []byte("v"), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(ctx, "k")
	if !errors.Is(err, kvstorage.ErrKeyNotFound) {
		t.Errorf("Get after Delete: error = %v, want ErrKeyNotFound", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Delete nonexistent should fail")
	}
	if !errors.Is(err, kvstorage.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestList_Empty(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	keys, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("List = %v, want empty", keys)
	}
}

func TestList(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	for _, k := range []string{"c", "a", "b"} {
		if err := s.Set(ctx, k, []byte("v"), kvstorage.SetOptions{}); err != nil {
			t.Fatalf("Set(%q): %v", k, err)
		}
	}

	keys, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("List returned %d keys, want 3", len(keys))
	}

	// Keys should be returned (ReadDir returns alphabetical order)
	want := []string{"a", "b", "c"}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, want[i])
		}
	}
}

func TestList_IgnoresNonJSON(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	if err := s.Set(ctx, "valid", []byte("v"), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Create a non-JSON file that should be ignored
	if err := os.WriteFile(filepath.Join(s.dir, "readme.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("write non-JSON: %v", err)
	}
	// Create a subdirectory that should be ignored
	if err := os.Mkdir(filepath.Join(s.dir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	keys, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0] != "valid" {
		t.Errorf("List = %v, want [valid]", keys)
	}
}

func TestValidateKey_Empty(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	err := s.Set(ctx, "", []byte("v"), kvstorage.SetOptions{})
	if err == nil {
		t.Fatal("empty key should fail")
	}
}

func TestValidateKey_PathSeparator(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	for _, key := range []string{"a/b", "a\\b"} {
		err := s.Set(ctx, key, []byte("v"), kvstorage.SetOptions{})
		if err == nil {
			t.Errorf("key %q with path separator should fail", key)
		}
	}
}

func TestAtomicWrite(t *testing.T) {
	s := newTestStore(t, "test")
	ctx := context.Background()

	// Write and verify no temp files remain
	if err := s.Set(ctx, "k", []byte("v"), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			t.Errorf("unexpected file: %s (temp file not cleaned up?)", e.Name())
		}
	}
}

func TestInit_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, "newtable")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Directory shouldn't exist yet
	tablePath := filepath.Join(dir, "newtable")
	if _, err := os.Stat(tablePath); !os.IsNotExist(err) {
		t.Fatal("table directory should not exist before Init")
	}

	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	info, err := os.Stat(tablePath)
	if err != nil {
		t.Fatalf("table directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("table path is not a directory")
	}
}

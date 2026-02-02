package yamlstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.All(); len(got) != 0 {
		t.Errorf("empty store All() = %v, want empty map", got)
	}
}

func TestNewLoadsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "actor: alice\ndefaults.priority: high\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	v, ok := s.Get("actor")
	if !ok || v != "alice" {
		t.Errorf("Get(actor) = %q, %v; want %q, true", v, ok, "alice")
	}
	v, ok = s.Get("defaults.priority")
	if !ok || v != "high" {
		t.Errorf("Get(defaults.priority) = %q, %v; want %q, true", v, ok, "high")
	}
}

func TestNewEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := s.All(); len(got) != 0 {
		t.Errorf("empty file All() = %v, want empty map", got)
	}
}

func TestGetMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	v, ok := s.Get("nonexistent")
	if ok {
		t.Errorf("Get(nonexistent) ok = true, want false")
	}
	if v != "" {
		t.Errorf("Get(nonexistent) = %q, want empty", v)
	}
}

func TestSetAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Set("defaults.type", "bug"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok := s.Get("defaults.type")
	if !ok || v != "bug" {
		t.Errorf("Get(defaults.type) = %q, %v; want %q, true", v, ok, "bug")
	}
}

func TestSetOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Set("actor", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("actor", "bob"); err != nil {
		t.Fatal(err)
	}

	v, ok := s.Get("actor")
	if !ok || v != "bob" {
		t.Errorf("Get(actor) = %q, %v; want %q, true", v, ok, "bob")
	}
}

func TestUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Set("actor", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.Unset("actor"); err != nil {
		t.Fatalf("Unset: %v", err)
	}

	_, ok := s.Get("actor")
	if ok {
		t.Error("Get(actor) ok = true after Unset, want false")
	}
}

func TestUnsetNonexistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Unset("nonexistent"); err != nil {
		t.Errorf("Unset(nonexistent) = %v, want nil", err)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Set("actor", "alice"); err != nil {
		t.Fatal(err)
	}

	all := s.All()
	all["actor"] = "MUTATED"

	v, _ := s.Get("actor")
	if v != "alice" {
		t.Errorf("mutation of All() result affected store: Get(actor) = %q, want %q", v, "alice")
	}
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s1, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s1.Set("actor", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Set("defaults.priority", "high"); err != nil {
		t.Fatal(err)
	}

	// Open a fresh store from the same file.
	s2, err := New(path)
	if err != nil {
		t.Fatalf("New (reload): %v", err)
	}

	v, ok := s2.Get("actor")
	if !ok || v != "alice" {
		t.Errorf("reloaded Get(actor) = %q, %v; want %q, true", v, ok, "alice")
	}
	v, ok = s2.Get("defaults.priority")
	if !ok || v != "high" {
		t.Errorf("reloaded Get(defaults.priority) = %q, %v; want %q, true", v, ok, "high")
	}
}

func TestPersistenceAfterUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s1, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s1.Set("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Set("b", "2"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Unset("a"); err != nil {
		t.Fatal(err)
	}

	s2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := s2.Get("a"); ok {
		t.Error("reloaded Get(a) ok = true after Unset, want false")
	}
	v, ok := s2.Get("b")
	if !ok || v != "2" {
		t.Errorf("reloaded Get(b) = %q, %v; want %q, true", v, ok, "2")
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	keys := map[string]string{
		"actor":             "alice",
		"defaults.priority": "high",
		"defaults.type":     "bug",
		"id.prefix":         "test-",
		"project.name":      "myproject",
	}
	for k, v := range keys {
		if err := s.Set(k, v); err != nil {
			t.Fatalf("Set(%q, %q): %v", k, v, err)
		}
	}

	// Reload and verify all keys.
	s2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	all := s2.All()
	if len(all) != len(keys) {
		t.Errorf("All() has %d entries, want %d", len(all), len(keys))
	}
	for k, want := range keys {
		got, ok := s2.Get(k)
		if !ok {
			t.Errorf("Get(%q) not found after round-trip", k)
			continue
		}
		if got != want {
			t.Errorf("Get(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestAlphabeticalOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	// Set in non-alphabetical order.
	if err := s.Set("z.key", "last"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("a.key", "first"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// yaml.Marshal on map[string]string produces alphabetical key ordering.
	want := "a.key: first\nz.key: last\n"
	if string(raw) != want {
		t.Errorf("file contents = %q, want %q", string(raw), want)
	}
}

func TestSetCreatesParentDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "config.yaml")

	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Set("key", "value"); err != nil {
		t.Fatalf("Set with nested path: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestConcurrentSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := New(path)
			if err != nil {
				errs[i] = err
				return
			}
			errs[i] = s.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("val%d", i))
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}

	// Reload and verify all keys present.
	s, err := New(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key%d", i)
		val, ok := s.Get(key)
		if !ok {
			t.Errorf("key %q missing after concurrent writes", key)
		} else if val != fmt.Sprintf("val%d", i) {
			t.Errorf("key %q = %q, want %q", key, val, fmt.Sprintf("val%d", i))
		}
	}
}

func TestUnsetSurvivesMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	s1, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Set("keep", "yes"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Set("remove", "yes"); err != nil {
		t.Fatal(err)
	}

	// A second store writes a third key (simulating another process).
	s2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Set("other", "val"); err != nil {
		t.Fatal(err)
	}

	// First store unsets "remove" — should re-read disk (seeing "other")
	// and delete "remove".
	if err := s1.Unset("remove"); err != nil {
		t.Fatal(err)
	}

	// Reload and verify.
	s3, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := s3.Get("remove"); ok {
		t.Error("key 'remove' should not exist after Unset")
	}
	if v, ok := s3.Get("keep"); !ok || v != "yes" {
		t.Errorf("Get(keep) = %q, %v; want 'yes', true", v, ok)
	}
	if v, ok := s3.Get("other"); !ok || v != "val" {
		t.Errorf("Get(other) = %q, %v; want 'val', true", v, ok)
	}
}

func TestNewInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(":::invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := New(path)
	if err == nil {
		t.Error("New with invalid YAML should return error")
	}
}

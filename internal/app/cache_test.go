package app

import (
	"os"
	"testing"
	"time"
)

func TestCacheReadWriteFresh(t *testing.T) {
	dir := t.TempDir()
	store := newCacheStore(Options{
		CacheEnabled:      true,
		CacheDir:          dir,
		CacheMode:         "balanced",
		CacheTTLAccounts:  time.Minute,
		CacheTTLRoles:     time.Minute,
		CacheTTLRegions:   time.Minute,
		CacheTTLInstances: time.Minute,
	})

	key := cacheKeyAccounts("prof", "us-east-1")
	payload := []string{"111111111111"}
	if err := store.writeJSON("prof", key, time.Minute, payload); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	var got []string
	status, _, err := store.readJSON("prof", key, &got)
	if err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	if status != cacheHitFresh {
		t.Fatalf("expected fresh hit, got %v", status)
	}
	if len(got) != 1 || got[0] != "111111111111" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestCacheExpiryReturnsStale(t *testing.T) {
	dir := t.TempDir()
	store := newCacheStore(Options{
		CacheEnabled: true,
		CacheDir:     dir,
		CacheMode:    "balanced",
	})

	key := cacheKeyRegions("prof", "swamp-1", "us-east-1", false)
	if err := store.writeJSON("prof", key, 10*time.Millisecond, []string{"us-east-1"}); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	var got []string
	status, _, err := store.readJSON("prof", key, &got)
	if err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	if status != cacheHitStale {
		t.Fatalf("expected stale hit, got %v", status)
	}
}

func TestCacheFreshModeBypassesRead(t *testing.T) {
	dir := t.TempDir()
	store := newCacheStore(Options{
		CacheEnabled: true,
		CacheDir:     dir,
		CacheMode:    "fresh",
	})
	key := cacheKeyAccounts("prof", "us-east-1")
	if err := store.writeJSON("prof", key, time.Minute, []string{"x"}); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	var got []string
	status, _, err := store.readJSON("prof", key, &got)
	if err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	if status != cacheMiss {
		t.Fatalf("expected miss in fresh mode, got %v", status)
	}
}

func TestCacheInvalidJSONIsIgnoredAndRemoved(t *testing.T) {
	dir := t.TempDir()
	store := newCacheStore(Options{
		CacheEnabled: true,
		CacheDir:     dir,
		CacheMode:    "balanced",
	})

	key := cacheKeyRoles("prof", "us-east-1", "123456789012")
	path := store.filePath("prof", key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	var got []string
	status, _, err := store.readJSON("prof", key, &got)
	if err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	if status != cacheMiss {
		t.Fatalf("expected miss, got %v", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected invalid cache file to be removed")
	}
}

func TestCacheKeyTemplates(t *testing.T) {
	if got := cacheKeyAccounts("p", "r"); got != "accounts:p:r" {
		t.Fatalf("unexpected accounts key: %s", got)
	}
	if got := cacheKeyRoles("p", "r", "a"); got != "roles:p:r:a" {
		t.Fatalf("unexpected roles key: %s", got)
	}
	if got := cacheKeyRegions("p", "dp", "dr", true); got != "regions:p:dp:dr:true" {
		t.Fatalf("unexpected regions key: %s", got)
	}
	if got := cacheKeyInstances("p", "a", "role", "us-east-1", false); got != "instances:p:a:role:us-east-1:false" {
		t.Fatalf("unexpected instances key: %s", got)
	}
}

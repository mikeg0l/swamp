package app

import (
	"strings"
	"testing"
	"time"
)

func TestValidateOptionsCacheModeValidation(t *testing.T) {
	err := validateOptions(Options{
		Profile:           "p",
		Workers:           1,
		CacheEnabled:      true,
		CacheDir:          "/tmp/swamp-cache",
		CacheMode:         "invalid",
		CacheTTLAccounts:  time.Minute,
		CacheTTLRoles:     time.Minute,
		CacheTTLRegions:   time.Minute,
		CacheTTLInstances: time.Minute,
	})
	if err == nil || !strings.Contains(err.Error(), "--cache-mode") {
		t.Fatalf("expected cache mode validation error, got %v", err)
	}
}

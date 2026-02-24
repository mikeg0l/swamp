package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadUserConfigMissingFileReturnsEmpty(t *testing.T) {
	cfg, err := loadUserConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Profile != "" || cfg.PreferredRole != "" {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}

func TestLoadUserConfigInvalidDurationRejectedInMerge(t *testing.T) {
	cfg := UserConfig{}
	cfg.Cache.TTLAccounts = "not-a-duration"
	_, err := mergeOptions(Options{Workers: 1}, cfg)
	if err == nil || !strings.Contains(err.Error(), "cache.ttl_accounts") {
		t.Fatalf("expected ttl parse error, got %v", err)
	}
}

func TestMergeOptionsPrecedenceFlagsOverConfig(t *testing.T) {
	flagSet := map[string]bool{
		"profile":            true,
		"role":               false,
		"cache-ttl-accounts": false,
		"workers":            true,
	}
	cli := Options{
		Profile:          "flag-profile",
		Workers:          33,
		CacheTTLAccounts: 2 * time.Hour,
		FlagSet:          flagSet,
	}
	cfg := UserConfig{
		Profile:       "config-profile",
		PreferredRole: "Admin",
		Discovery:     userConfigDisc{Workers: intPtr(12)},
		Cache:         userConfigCache{TTLAccounts: "6h"},
	}

	got, err := mergeOptions(cli, cfg)
	if err != nil {
		t.Fatalf("mergeOptions failed: %v", err)
	}
	if got.Profile != "flag-profile" {
		t.Fatalf("expected flag profile to win, got %q", got.Profile)
	}
	if got.Workers != 33 {
		t.Fatalf("expected flag workers to win, got %d", got.Workers)
	}
	if got.RoleFilter != "Admin" || !got.RoleFromPreferred {
		t.Fatalf("expected preferred role from config, got role=%q preferred=%t", got.RoleFilter, got.RoleFromPreferred)
	}
	if got.CacheTTLAccounts != 6*time.Hour {
		t.Fatalf("expected config ttl to apply, got %s", got.CacheTTLAccounts)
	}
}

func TestMergeOptionsSkipRegionSelectFromConfig(t *testing.T) {
	cli := Options{
		Workers: 1,
		FlagSet: map[string]bool{
			"skip-region-select": false,
		},
	}
	cfg := UserConfig{
		UX: userConfigUX{
			SkipRegionSelect: boolPtr(true),
		},
	}

	got, err := mergeOptions(cli, cfg)
	if err != nil {
		t.Fatalf("mergeOptions failed: %v", err)
	}
	if !got.SkipRegionSelect {
		t.Fatal("expected SkipRegionSelect to be set from config")
	}
	if src := got.ValueSource["skip-region-select"]; src != "config(ux.skip_region_select)" {
		t.Fatalf("unexpected source for skip-region-select: %q", src)
	}
}

func TestResolveConfigPathDefault(t *testing.T) {
	got := resolveConfigPath("")
	if !strings.Contains(got, ".config/swamp/config.yaml") {
		t.Fatalf("unexpected default config path: %s", got)
	}
}

func TestWriteConfigExample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := writeConfigExample(path); err != nil {
		t.Fatalf("writeConfigExample failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if !strings.Contains(string(data), "preferred_role:") {
		t.Fatalf("unexpected config content: %s", string(data))
	}
}

func TestEnsureDefaultConfigFileCreatesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := ensureDefaultConfigFile(path); err != nil {
		t.Fatalf("ensureDefaultConfigFile failed: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if !strings.Contains(string(first), "cache:") {
		t.Fatalf("expected default config content, got: %s", string(first))
	}

	// second call should not overwrite
	if err := os.WriteFile(path, []byte("profile: keep\n"), 0o644); err != nil {
		t.Fatalf("overwrite setup failed: %v", err)
	}
	if err := ensureDefaultConfigFile(path); err != nil {
		t.Fatalf("ensureDefaultConfigFile second call failed: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if string(second) != "profile: keep\n" {
		t.Fatalf("expected existing config to be preserved, got: %s", string(second))
	}
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

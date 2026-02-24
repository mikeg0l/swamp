package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type cacheStore struct {
	cfg cacheConfig
	mu  sync.Mutex
	sem chan struct{}
}

func defaultCacheDir() string {
	d, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(d) == "" {
		home, herr := os.UserHomeDir()
		if herr != nil || strings.TrimSpace(home) == "" {
			return ".swamp-cache"
		}
		return filepath.Join(home, ".cache", "swamp")
	}
	return filepath.Join(d, "swamp")
}

func DefaultCacheDirForCLI() string {
	return defaultCacheDir()
}

func newCacheStore(opts Options) *cacheStore {
	mode := cacheMode(strings.ToLower(strings.TrimSpace(opts.CacheMode)))
	if mode != cacheModeBalanced && mode != cacheModeFresh && mode != cacheModeSpeed {
		mode = cacheModeBalanced
	}
	return &cacheStore{
		cfg: cacheConfig{
			Enabled:      opts.CacheEnabled,
			Dir:          opts.CacheDir,
			Mode:         mode,
			TTLAccounts:  opts.CacheTTLAccounts,
			TTLRoles:     opts.CacheTTLRoles,
			TTLRegions:   opts.CacheTTLRegions,
			TTLInstances: opts.CacheTTLInstances,
		},
		sem: make(chan struct{}, 8),
	}
}

func (c *cacheStore) isEnabled() bool {
	return c != nil && c.cfg.Enabled
}

func (c *cacheStore) shouldBypassRead() bool {
	return !c.isEnabled() || c.cfg.Mode == cacheModeFresh
}

func (c *cacheStore) shouldUseStale() bool {
	return c.isEnabled() && (c.cfg.Mode == cacheModeBalanced || c.cfg.Mode == cacheModeSpeed)
}

func (c *cacheStore) readJSON(profile, key string, out any) (cacheReadStatus, time.Duration, error) {
	if c.shouldBypassRead() {
		return cacheMiss, 0, nil
	}
	env, err := c.readEnvelope(profile, key)
	if err != nil {
		return cacheMiss, 0, err
	}
	if env == nil {
		return cacheMiss, 0, nil
	}
	if err := json.Unmarshal(env.Payload, out); err != nil {
		return cacheMiss, 0, nil
	}

	age := time.Since(env.CreatedAt)
	if time.Now().After(env.ExpiresAt) {
		return cacheHitStale, age, nil
	}
	return cacheHitFresh, age, nil
}

func (c *cacheStore) writeJSON(profile, key string, ttl time.Duration, payload any) error {
	if !c.isEnabled() {
		return nil
	}
	if ttl <= 0 {
		return nil
	}
	if err := os.MkdirAll(c.cfg.Dir, 0o755); err != nil {
		return err
	}

	blob, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	env := cacheEnvelope{
		Version:   cacheVersion,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		Profile:   profile,
		Key:       key,
		Payload:   blob,
	}
	content, err := json.Marshal(env)
	if err != nil {
		return err
	}

	path := c.filePath(profile, key)
	tmp, err := os.CreateTemp(c.cfg.Dir, "swamp-cache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (c *cacheStore) readEnvelope(profile, key string) (*cacheEnvelope, error) {
	if !c.isEnabled() {
		return nil, nil
	}
	path := c.filePath(profile, key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var env cacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		_ = os.Remove(path)
		return nil, nil
	}
	if env.Version != cacheVersion || env.Key != key || env.Profile != profile {
		_ = os.Remove(path)
		return nil, nil
	}
	return &env, nil
}

func (c *cacheStore) filePath(profile, key string) string {
	h := sha256.Sum256([]byte(profile + "|" + key))
	filename := fmt.Sprintf("%s.json", hex.EncodeToString(h[:]))
	return filepath.Join(c.cfg.Dir, filename)
}

func (c *cacheStore) clear() error {
	if !c.isEnabled() {
		return nil
	}
	if strings.TrimSpace(c.cfg.Dir) == "" {
		return nil
	}
	if _, err := os.Stat(c.cfg.Dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(c.cfg.Dir)
}

func (c *cacheStore) refreshAsync(fn func() error) {
	if !c.isEnabled() {
		return
	}
	select {
	case c.sem <- struct{}{}:
	default:
		return
	}
	go func() {
		defer func() { <-c.sem }()
		_ = fn()
	}()
}

func cacheKeyAccounts(profile, ssoRegion string) string {
	return fmt.Sprintf("accounts:%s:%s", profile, ssoRegion)
}

func cacheKeyRoles(profile, ssoRegion, accountID string) string {
	return fmt.Sprintf("roles:%s:%s:%s", profile, ssoRegion, accountID)
}

func cacheKeyRegions(profile, discoveryProfile, discoveryRegion string, includeAllRegions bool) string {
	return fmt.Sprintf("regions:%s:%s:%s:%t", profile, discoveryProfile, discoveryRegion, includeAllRegions)
}

func cacheKeyInstances(profile, accountID, role, region string, runningOnly bool) string {
	return fmt.Sprintf("instances:%s:%s:%s:%s:%t", profile, accountID, role, region, runningOnly)
}

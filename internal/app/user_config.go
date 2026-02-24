package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultConfigRelPath = ".config/swamp/config.yaml"

type UserConfig struct {
	Profile       string          `yaml:"profile"`
	PreferredRole string          `yaml:"preferred_role"`
	Cache         userConfigCache `yaml:"cache"`
	Discovery     userConfigDisc  `yaml:"discovery"`
	UX            userConfigUX    `yaml:"ux"`
}

type userConfigCache struct {
	Enabled      *bool  `yaml:"enabled"`
	Dir          string `yaml:"dir"`
	Mode         string `yaml:"mode"`
	TTLAccounts  string `yaml:"ttl_accounts"`
	TTLRoles     string `yaml:"ttl_roles"`
	TTLRegions   string `yaml:"ttl_regions"`
	TTLInstances string `yaml:"ttl_instances"`
}

type userConfigDisc struct {
	Workers        *int     `yaml:"workers"`
	Regions        []string `yaml:"regions"`
	AllRegions     *bool    `yaml:"all_regions"`
	IncludeStopped *bool    `yaml:"include_stopped"`
}

type userConfigUX struct {
	AutoSelectSingle *bool `yaml:"auto_select_single"`
	ResumeByDefault  *bool `yaml:"resume_by_default"`
}

func resolveConfigPath(cliPath string) string {
	if strings.TrimSpace(cliPath) != "" {
		return expandTilde(cliPath)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return defaultConfigRelPath
	}
	return filepath.Join(home, defaultConfigRelPath)
}

func loadUserConfig(path string) (UserConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return UserConfig{}, nil
		}
		return UserConfig{}, fmt.Errorf("read config file %q: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return UserConfig{}, nil
	}

	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return UserConfig{}, fmt.Errorf("parse config file %q: %w", path, err)
	}
	warnUnknownConfigKeys(root)

	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return UserConfig{}, fmt.Errorf("decode config file %q: %w", path, err)
	}
	return cfg, nil
}

func ensureDefaultConfigFile(path string) error {
	target := resolveConfigPath(path)
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config file %q: %w", target, err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	content := defaultConfigContent()
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write default config file %q: %w", target, err)
	}
	fmt.Printf("Created default config at %s\n", target)
	return nil
}

func mergeOptions(cli Options, cfg UserConfig) (Options, error) {
	out := cli
	sources := map[string]string{
		"profile":             "built-in",
		"workers":             "built-in",
		"account":             "built-in",
		"role":                "built-in",
		"regions":             "built-in",
		"all-regions":         "built-in",
		"include-stopped":     "built-in",
		"cache":               "built-in",
		"cache-dir":           "built-in",
		"cache-mode":          "built-in",
		"cache-ttl-accounts":  "built-in",
		"cache-ttl-roles":     "built-in",
		"cache-ttl-regions":   "built-in",
		"cache-ttl-instances": "built-in",
		"resume":              "built-in",
		"last":                "built-in",
		"no-auto-select":      "built-in",
	}

	setFromConfig := func(name string) bool {
		return !cli.flagChanged(name)
	}
	setFromFlag := func(name, sourceKey string) {
		if cli.flagChanged(name) {
			sources[sourceKey] = "flag"
		}
	}

	if setFromConfig("profile") && strings.TrimSpace(cfg.Profile) != "" {
		out.Profile = strings.TrimSpace(cfg.Profile)
		sources["profile"] = "config"
	}
	if setFromConfig("role") && strings.TrimSpace(cfg.PreferredRole) != "" {
		out.RoleFilter = strings.TrimSpace(cfg.PreferredRole)
		out.RoleFromPreferred = true
		sources["role"] = "config(preferred_role)"
	}
	if setFromConfig("workers") && cfg.Discovery.Workers != nil {
		out.Workers = *cfg.Discovery.Workers
		sources["workers"] = "config"
	}
	if setFromConfig("regions") && len(cfg.Discovery.Regions) > 0 {
		out.RegionsArg = strings.Join(cfg.Discovery.Regions, ",")
		sources["regions"] = "config"
	}
	if setFromConfig("all-regions") && cfg.Discovery.AllRegions != nil {
		out.AllRegions = *cfg.Discovery.AllRegions
		sources["all-regions"] = "config"
	}
	if setFromConfig("include-stopped") && cfg.Discovery.IncludeStopped != nil {
		out.IncludeStopped = *cfg.Discovery.IncludeStopped
		sources["include-stopped"] = "config"
	}
	if setFromConfig("cache") && cfg.Cache.Enabled != nil {
		out.CacheEnabled = *cfg.Cache.Enabled
		sources["cache"] = "config"
	}
	if setFromConfig("cache-dir") && strings.TrimSpace(cfg.Cache.Dir) != "" {
		out.CacheDir = expandTilde(strings.TrimSpace(cfg.Cache.Dir))
		sources["cache-dir"] = "config"
	}
	if setFromConfig("cache-mode") && strings.TrimSpace(cfg.Cache.Mode) != "" {
		out.CacheMode = strings.TrimSpace(cfg.Cache.Mode)
		sources["cache-mode"] = "config"
	}

	var err error
	if setFromConfig("cache-ttl-accounts") && strings.TrimSpace(cfg.Cache.TTLAccounts) != "" {
		out.CacheTTLAccounts, err = parseConfigDuration("cache.ttl_accounts", cfg.Cache.TTLAccounts)
		if err != nil {
			return Options{}, err
		}
		sources["cache-ttl-accounts"] = "config"
	}
	if setFromConfig("cache-ttl-roles") && strings.TrimSpace(cfg.Cache.TTLRoles) != "" {
		out.CacheTTLRoles, err = parseConfigDuration("cache.ttl_roles", cfg.Cache.TTLRoles)
		if err != nil {
			return Options{}, err
		}
		sources["cache-ttl-roles"] = "config"
	}
	if setFromConfig("cache-ttl-regions") && strings.TrimSpace(cfg.Cache.TTLRegions) != "" {
		out.CacheTTLRegions, err = parseConfigDuration("cache.ttl_regions", cfg.Cache.TTLRegions)
		if err != nil {
			return Options{}, err
		}
		sources["cache-ttl-regions"] = "config"
	}
	if setFromConfig("cache-ttl-instances") && strings.TrimSpace(cfg.Cache.TTLInstances) != "" {
		out.CacheTTLInstances, err = parseConfigDuration("cache.ttl_instances", cfg.Cache.TTLInstances)
		if err != nil {
			return Options{}, err
		}
		sources["cache-ttl-instances"] = "config"
	}
	if setFromConfig("no-auto-select") && cfg.UX.AutoSelectSingle != nil {
		out.NoAutoSelect = !*cfg.UX.AutoSelectSingle
		sources["no-auto-select"] = "config(ux.auto_select_single)"
	}
	if setFromConfig("resume") && cfg.UX.ResumeByDefault != nil {
		out.Resume = *cfg.UX.ResumeByDefault
		sources["resume"] = "config(ux.resume_by_default)"
	}

	if cli.flagChanged("role") {
		out.RoleFromPreferred = false
	}

	setFromFlag("profile", "profile")
	setFromFlag("workers", "workers")
	setFromFlag("account", "account")
	setFromFlag("role", "role")
	setFromFlag("regions", "regions")
	setFromFlag("all-regions", "all-regions")
	setFromFlag("include-stopped", "include-stopped")
	setFromFlag("cache", "cache")
	setFromFlag("cache-dir", "cache-dir")
	setFromFlag("cache-mode", "cache-mode")
	setFromFlag("cache-ttl-accounts", "cache-ttl-accounts")
	setFromFlag("cache-ttl-roles", "cache-ttl-roles")
	setFromFlag("cache-ttl-regions", "cache-ttl-regions")
	setFromFlag("cache-ttl-instances", "cache-ttl-instances")
	setFromFlag("resume", "resume")
	setFromFlag("last", "last")
	setFromFlag("no-auto-select", "no-auto-select")

	out.ValueSource = sources
	return out, nil
}

func printEffectiveConfig(opts Options) {
	fmt.Printf("profile: %s\n", opts.Profile)
	fmt.Printf("role: %s\n", opts.RoleFilter)
	fmt.Printf("workers: %d\n", opts.Workers)
	fmt.Printf("regions: %s\n", opts.RegionsArg)
	fmt.Printf("all_regions: %t\n", opts.AllRegions)
	fmt.Printf("include_stopped: %t\n", opts.IncludeStopped)
	fmt.Printf("resume: %t\n", opts.Resume)
	fmt.Printf("last: %t\n", opts.Last)
	fmt.Printf("no_auto_select: %t\n", opts.NoAutoSelect)
	fmt.Printf("cache.enabled: %t\n", opts.CacheEnabled)
	fmt.Printf("cache.dir: %s\n", opts.CacheDir)
	fmt.Printf("cache.mode: %s\n", opts.CacheMode)
	fmt.Printf("cache.ttl_accounts: %s\n", opts.CacheTTLAccounts)
	fmt.Printf("cache.ttl_roles: %s\n", opts.CacheTTLRoles)
	fmt.Printf("cache.ttl_regions: %s\n", opts.CacheTTLRegions)
	fmt.Printf("cache.ttl_instances: %s\n", opts.CacheTTLInstances)
}

func configExample() string {
	return renderConfigYAML("my-sso-profile", "AdministratorAccess")
}

func defaultConfigContent() string {
	return renderConfigYAML("", "")
}

func renderConfigYAML(profile, preferredRole string) string {
	return fmt.Sprintf(`profile: %s
preferred_role: %s

cache:
  enabled: true
  dir: %s
  mode: balanced
  ttl_accounts: 6h
  ttl_roles: 6h
  ttl_regions: 24h
  ttl_instances: 60s

discovery:
  workers: 12
  regions: []
  all_regions: false
  include_stopped: false

ux:
  auto_select_single: true
  resume_by_default: false
`, strconv.Quote(profile), strconv.Quote(preferredRole), strconv.Quote(defaultCacheDir()))
}

func writeConfigExample(path string) error {
	if strings.TrimSpace(path) == "-" {
		fmt.Print(configExample())
		return nil
	}
	target := resolveConfigPath(path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("config file already exists at %s", target)
	}
	if err := os.WriteFile(target, []byte(configExample()), 0o644); err != nil {
		return fmt.Errorf("write config file %q: %w", target, err)
	}
	fmt.Printf("Wrote example config to %s\n", target)
	return nil
}

func parseConfigDuration(key, value string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid config value %s=%q: %w", key, value, err)
	}
	return d, nil
}

func warnUnknownConfigKeys(root map[string]any) {
	knownRoot := map[string]struct{}{
		"profile":        {},
		"preferred_role": {},
		"cache":          {},
		"discovery":      {},
		"ux":             {},
	}
	knownCache := map[string]struct{}{
		"enabled":       {},
		"dir":           {},
		"mode":          {},
		"ttl_accounts":  {},
		"ttl_roles":     {},
		"ttl_regions":   {},
		"ttl_instances": {},
	}
	knownDiscovery := map[string]struct{}{
		"workers":         {},
		"regions":         {},
		"all_regions":     {},
		"include_stopped": {},
	}
	knownUX := map[string]struct{}{
		"auto_select_single": {},
		"resume_by_default":  {},
	}

	for k, v := range root {
		if _, ok := knownRoot[k]; !ok {
			fmt.Fprintf(os.Stderr, "warning: ignoring unknown config key %q\n", k)
			continue
		}
		switch k {
		case "cache":
			warnUnknownNested("cache", v, knownCache)
		case "discovery":
			warnUnknownNested("discovery", v, knownDiscovery)
		case "ux":
			warnUnknownNested("ux", v, knownUX)
		}
	}
}

func warnUnknownNested(prefix string, raw any, known map[string]struct{}) {
	m, ok := raw.(map[string]any)
	if !ok {
		return
	}
	for k := range m {
		if _, exists := known[k]; !exists {
			fmt.Fprintf(os.Stderr, "warning: ignoring unknown config key %q\n", prefix+"."+k)
		}
	}
}

func expandTilde(path string) string {
	p := strings.TrimSpace(path)
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func sourceOf(opts Options, key string) string {
	if opts.ValueSource == nil {
		return "unknown"
	}
	if src, ok := opts.ValueSource[key]; ok {
		return src
	}
	return "unknown"
}

func validateOptionsWithSource(opts Options) error {
	if err := validateOptions(opts); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "--workers"):
			return fmt.Errorf("%s (source=%s)", msg, sourceOf(opts, "workers"))
		case strings.Contains(msg, "--cache-dir"):
			return fmt.Errorf("%s (source=%s)", msg, sourceOf(opts, "cache-dir"))
		case strings.Contains(msg, "cache TTL"):
			return fmt.Errorf("%s (sources accounts=%s roles=%s regions=%s instances=%s)",
				msg,
				sourceOf(opts, "cache-ttl-accounts"),
				sourceOf(opts, "cache-ttl-roles"),
				sourceOf(opts, "cache-ttl-regions"),
				sourceOf(opts, "cache-ttl-instances"),
			)
		case strings.Contains(msg, "--cache-mode"):
			return fmt.Errorf("%s (source=%s)", msg, sourceOf(opts, "cache-mode"))
		case strings.Contains(msg, "--profile"):
			return errors.New("missing profile: set --profile, or configure profile in config file")
		default:
			return err
		}
	}
	return nil
}

func (o Options) flagChanged(name string) bool {
	if o.FlagSet == nil {
		return false
	}
	return o.FlagSet[name]
}

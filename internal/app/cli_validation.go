package app

import (
	"errors"
	"os/exec"
	"strings"
)

func validateOptions(opts Options) error {
	if opts.Profile == "" {
		return errors.New("missing --profile/-p (example: --profile my-sso-profile)")
	}
	if opts.Workers < 1 {
		return errors.New("--workers must be at least 1")
	}
	if opts.CacheEnabled {
		if strings.TrimSpace(opts.CacheDir) == "" {
			return errors.New("--cache-dir must not be empty when --cache=true")
		}
		if opts.CacheTTLAccounts < 0 || opts.CacheTTLRoles < 0 || opts.CacheTTLRegions < 0 || opts.CacheTTLInstances < 0 {
			return errors.New("cache TTL values must be non-negative")
		}
		switch strings.ToLower(strings.TrimSpace(opts.CacheMode)) {
		case "balanced", "fresh", "speed":
		default:
			return errors.New("--cache-mode must be one of: balanced, fresh, speed")
		}
	}
	return nil
}

func validateDependencies() error {
	if _, err := exec.LookPath("aws"); err != nil {
		return errors.New("aws CLI not found in PATH")
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return errors.New("fzf not found in PATH")
	}
	return nil
}

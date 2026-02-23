package app

import (
	"errors"
	"os/exec"
)

func validateOptions(opts Options) error {
	if opts.Profile == "" {
		return errors.New("missing --profile (example: --profile my-sso-profile)")
	}
	if opts.Workers < 1 {
		return errors.New("--workers must be at least 1")
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

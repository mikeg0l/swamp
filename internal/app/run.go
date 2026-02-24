package app

import (
	"errors"
	"fmt"
	"os"
	"sort"
)

func Run(opts Options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	if err := validateDependencies(); err != nil {
		return err
	}
	opts.cacheStore = newCacheStore(opts)
	if opts.CacheClear {
		if err := opts.cacheStore.clear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Printf("Cleared cache at %s\n", opts.CacheDir)
	}

	cfg, err := readProfileConfig(opts.Profile)
	if err != nil {
		return fmt.Errorf("failed to read profile config: %w", err)
	}
	if !cfg.SourceExists {
		return fmt.Errorf("profile %q was not found in ~/.aws/config", opts.Profile)
	}

	fmt.Printf("Checking SSO session for profile %q...\n", opts.Profile)
	accessToken, err := ensureSSOLoginAndGetToken(opts.Profile, cfg.SSOStartURL)
	if err != nil {
		return fmt.Errorf("failed to authenticate profile %q: %w", opts.Profile, err)
	}
	ssoRegion := resolveSSORegion(cfg)

	accounts, err := discoverAccounts(opts, ssoRegion, accessToken)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return nil
	}

	targets, err := discoverRoleTargets(opts, accounts, ssoRegion, accessToken)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	fmt.Printf("Discovered %d account/role combinations\n", len(targets))

	tmpConfigPath, profileNames, err := buildTemporaryAWSConfig(cfg, targets)
	if err != nil {
		return fmt.Errorf("failed to build temporary AWS config: %w", err)
	}
	defer os.Remove(tmpConfigPath)

	regions, err := discoverRegions(opts, cfg, targets, tmpConfigPath, profileNames, ssoRegion)
	if err != nil {
		return err
	}
	if len(regions) == 0 {
		return nil
	}
	fmt.Printf("Scanning %d regions\n", len(regions))

	candidates := scanAllInstances(opts, tmpConfigPath, targets, profileNames, regions, opts.Workers, !opts.IncludeStopped)
	if len(candidates) == 0 {
		return errors.New("no EC2 instances found for the discovered account/role/region scope")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].DisplayLine < candidates[j].DisplayLine
	})

	selected, err := pickWithFZF(candidates)
	if err != nil {
		return fmt.Errorf("selection failed: %w", err)
	}
	if selected == nil {
		fmt.Println("No instance selected.")
		return nil
	}

	fmt.Printf("Starting SSM session to %s in %s (profile %s)\n", selected.InstanceID, selected.Region, selected.ProfileName)
	if err := startSSMSession(tmpConfigPath, selected.ProfileName, selected.Region, selected.InstanceID); err != nil {
		return fmt.Errorf("ssm session failed: %w", err)
	}
	return nil
}

package app

import (
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
	return runInteractiveScope(opts, cfg, ssoRegion, accessToken, accounts)
}

func runInteractiveScope(opts Options, cfg profileConfig, ssoRegion, accessToken string, accounts []ssoAccountsResponse) error {
	for {
		selectedAccount, err := selectAccountWithFZF(accounts)
		if err != nil {
			return fmt.Errorf("account selection failed: %w", err)
		}
		if selectedAccount == nil {
			fmt.Println("No account selected.")
			return nil
		}

		targets, err := discoverRoleTargets(opts, []ssoAccountsResponse{*selectedAccount}, ssoRegion, accessToken)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			continue
		}

		for {
			selectedTarget, backToAccounts, err := selectRoleTargetWithFZF(targets)
			if err != nil {
				return fmt.Errorf("role selection failed: %w", err)
			}
			if backToAccounts {
				break
			}
			if selectedTarget == nil {
				fmt.Println("No role selected.")
				return nil
			}

			selectedTargets := []roleTarget{*selectedTarget}
			tmpConfigPath, profileNames, err := buildTemporaryAWSConfig(cfg, selectedTargets)
			if err != nil {
				return fmt.Errorf("failed to build temporary AWS config: %w", err)
			}

			regions, err := discoverRegions(opts, cfg, selectedTargets, tmpConfigPath, profileNames, ssoRegion)
			if err != nil {
				_ = os.Remove(tmpConfigPath)
				return err
			}
			if len(regions) == 0 {
				_ = os.Remove(tmpConfigPath)
				continue
			}
			fmt.Printf("Scanning %d regions\n", len(regions))

			backToRoles := false
			for {
				selectedRegion, back, err := selectRegionWithFZF(regions)
				if err != nil {
					_ = os.Remove(tmpConfigPath)
					return fmt.Errorf("region selection failed: %w", err)
				}
				if back {
					backToRoles = true
					break
				}
				if selectedRegion == "" {
					fmt.Println("No region selected.")
					_ = os.Remove(tmpConfigPath)
					return nil
				}

				candidates := scanAllInstances(opts, tmpConfigPath, selectedTargets, profileNames, []string{selectedRegion}, opts.Workers, !opts.IncludeStopped)
				if len(candidates) == 0 {
					fmt.Printf("No EC2 instances found in %s.\n", selectedRegion)
					continue
				}
				sort.Slice(candidates, func(i, j int) bool {
					return candidates[i].DisplayLine < candidates[j].DisplayLine
				})

				selected, backToRegions, err := pickWithFZF(candidates)
				if err != nil {
					_ = os.Remove(tmpConfigPath)
					return fmt.Errorf("selection failed: %w", err)
				}
				if backToRegions {
					continue
				}
				if selected == nil {
					fmt.Println("No instance selected.")
					_ = os.Remove(tmpConfigPath)
					return nil
				}

				fmt.Printf("Starting SSM session to %s in %s (profile %s)\n", selected.InstanceID, selected.Region, selected.ProfileName)
				if err := startSSMSession(tmpConfigPath, selected.ProfileName, selected.Region, selected.InstanceID); err != nil {
					_ = os.Remove(tmpConfigPath)
					return fmt.Errorf("ssm session failed: %w", err)
				}
				_ = os.Remove(tmpConfigPath)
				return nil
			}

			_ = os.Remove(tmpConfigPath)
			if backToRoles {
				continue
			}
		}
	}
}

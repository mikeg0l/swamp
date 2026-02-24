package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

var (
	selectAccountFn       = selectAccountWithFZF
	discoverRoleTargetsFn = discoverRoleTargets
	selectRoleTargetFn    = selectRoleTargetWithFZF
	buildTempAWSConfigFn  = buildTemporaryAWSConfig
	discoverRegionsFn     = discoverRegions
	selectRegionFn        = selectRegionWithFZF
	scanAllInstancesFn    = scanAllInstances
	pickInstanceFn        = pickWithFZF
	startSSMSessionFn     = startSSMSession
	removeFileFn          = os.Remove
)

func Run(opts Options) error {
	resolvedOpts, cfg, err := resolveRuntimeOptions(opts)
	if err != nil {
		return err
	}
	if resolvedOpts.WriteConfigExample {
		return writeConfigExample(resolvedOpts.ConfigPath)
	}
	if resolvedOpts.PrintEffectiveConfig {
		printEffectiveConfig(resolvedOpts)
		return nil
	}

	if err := validateOptionsWithSource(resolvedOpts); err != nil {
		return err
	}
	if err := validateDependencies(); err != nil {
		return err
	}
	resolvedOpts.cacheStore = newCacheStore(resolvedOpts)
	if resolvedOpts.CacheClear {
		if err := resolvedOpts.cacheStore.clear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Printf("Cleared cache at %s\n", resolvedOpts.CacheDir)
	}

	if !cfg.SourceExists {
		return fmt.Errorf("profile %q was not found in ~/.aws/config", resolvedOpts.Profile)
	}

	fmt.Printf("Checking SSO session for profile %q...\n", resolvedOpts.Profile)
	accessToken, err := ensureSSOLoginAndGetToken(resolvedOpts.Profile, cfg.SSOStartURL)
	if err != nil {
		return fmt.Errorf("failed to authenticate profile %q: %w", resolvedOpts.Profile, err)
	}
	ssoRegion := resolveSSORegion(cfg)

	recent, recentErr := loadRecentTargets(resolvedOpts.CacheDir)
	if recentErr != nil {
		fmt.Printf("warning: failed to load recent targets: %v\n", recentErr)
		recent = recentTargetsFile{Version: 1, Profiles: map[string]recentProfileData{}}
	}

	if resolvedOpts.Last {
		ok, err := tryLastConnection(resolvedOpts, cfg, recent, ssoRegion)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	if resolvedOpts.Resume && resolvedOpts.AccountFilter == "" && resolvedOpts.RoleFilter == "" && resolvedOpts.RegionsArg == "" {
		if scope, ok := recent.getLastScope(resolvedOpts.Profile); ok {
			resolvedOpts.AccountFilter = scope.AccountID
			resolvedOpts.RoleFilter = scope.RoleName
			resolvedOpts.RegionsArg = scope.Region
			fmt.Printf("Resuming last scope: account=%s role=%s region=%s\n", scope.AccountID, scope.RoleName, scope.Region)
		} else {
			fmt.Printf("No recent scope found for profile %q; continuing interactively.\n", resolvedOpts.Profile)
		}
	}

	accounts, err := discoverAccounts(resolvedOpts, ssoRegion, accessToken)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return nil
	}
	return runInteractiveScope(resolvedOpts, cfg, ssoRegion, accessToken, accounts)
}

func resolveRuntimeOptions(opts Options) (Options, profileConfig, error) {
	configPath := resolveConfigPath(opts.ConfigPath)
	if opts.WriteConfigExample {
		opts.ConfigPath = configPath
		return opts, profileConfig{}, nil
	}
	if err := ensureDefaultConfigFile(configPath); err != nil {
		return Options{}, profileConfig{}, err
	}
	cfgFile, err := loadUserConfig(configPath)
	if err != nil {
		return Options{}, profileConfig{}, err
	}
	merged, err := mergeOptions(opts, cfgFile)
	if err != nil {
		return Options{}, profileConfig{}, err
	}
	merged.ConfigPath = configPath
	if merged.Last {
		merged.Resume = false
	}
	if merged.WriteConfigExample || merged.PrintEffectiveConfig || strings.TrimSpace(merged.Profile) == "" {
		return merged, profileConfig{}, nil
	}
	profileCfg, err := readProfileConfig(merged.Profile)
	if err != nil {
		return Options{}, profileConfig{}, fmt.Errorf("failed to read profile config: %w", err)
	}
	return merged, profileCfg, nil
}

func runInteractiveScope(opts Options, cfg profileConfig, ssoRegion, accessToken string, accounts []ssoAccountsResponse) error {
	for {
		var selectedAccount *ssoAccountsResponse
		var err error
		if !opts.NoAutoSelect && len(accounts) == 1 {
			selectedAccount = &accounts[0]
			if len(selectedAccount.AccountList) > 0 {
				fmt.Printf("Auto-selected only available account: %s\n", selectedAccount.AccountList[0].AccountID)
			}
		} else {
			selectedAccount, err = selectAccountFn(accounts)
		}
		if err != nil {
			return fmt.Errorf("account selection failed: %w", err)
		}
		if selectedAccount == nil {
			fmt.Println("No account selected.")
			return nil
		}

		targets, err := discoverRoleTargetsFn(opts, []ssoAccountsResponse{*selectedAccount}, ssoRegion, accessToken)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			continue
		}

		for {
			var selectedTarget *roleTarget
			var backToAccounts bool
			if !opts.NoAutoSelect && len(targets) == 1 {
				selectedTarget = &targets[0]
				fmt.Printf("Auto-selected only available role: %s\n", selectedTarget.RoleName)
			} else {
				selectedTarget, backToAccounts, err = selectRoleTargetFn(targets)
			}
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
			tmpConfigPath, profileNames, err := buildTempAWSConfigFn(cfg, selectedTargets)
			if err != nil {
				return fmt.Errorf("failed to build temporary AWS config: %w", err)
			}

			regions, err := discoverRegionsFn(opts, cfg, selectedTargets, tmpConfigPath, profileNames, ssoRegion)
			if err != nil {
				_ = removeFileFn(tmpConfigPath)
				return err
			}
			if len(regions) == 0 {
				_ = removeFileFn(tmpConfigPath)
				continue
			}
			fmt.Printf("Scanning %d regions\n", len(regions))

			backToRoles := false
			for {
				var selectedRegion string
				var back bool
				if !opts.NoAutoSelect && len(regions) == 1 {
					selectedRegion = regions[0]
					fmt.Printf("Auto-selected only available region: %s\n", selectedRegion)
				} else {
					selectedRegion, back, err = selectRegionFn(regions)
				}
				if err != nil {
					_ = removeFileFn(tmpConfigPath)
					return fmt.Errorf("region selection failed: %w", err)
				}
				if back {
					backToRoles = true
					break
				}
				if selectedRegion == "" {
					fmt.Println("No region selected.")
					_ = removeFileFn(tmpConfigPath)
					return nil
				}

				candidates := scanAllInstancesFn(opts, tmpConfigPath, selectedTargets, profileNames, []string{selectedRegion}, opts.Workers, !opts.IncludeStopped)
				if len(candidates) == 0 {
					fmt.Printf("No EC2 instances found in %s.\n", selectedRegion)
					continue
				}
				sort.Slice(candidates, func(i, j int) bool {
					return candidates[i].DisplayLine < candidates[j].DisplayLine
				})

				var selected *instanceCandidate
				var backToRegions bool
				if !opts.NoAutoSelect && len(candidates) == 1 {
					selected = &candidates[0]
					fmt.Printf("Auto-selected only available instance: %s\n", selected.InstanceID)
				} else {
					selected, backToRegions, err = pickInstanceFn(candidates)
				}
				if err != nil {
					_ = removeFileFn(tmpConfigPath)
					return fmt.Errorf("selection failed: %w", err)
				}
				if backToRegions {
					continue
				}
				if selected == nil {
					fmt.Println("No instance selected.")
					_ = removeFileFn(tmpConfigPath)
					return nil
				}

				fmt.Printf("Starting SSM session to %s in %s (profile %s)\n", selected.InstanceID, selected.Region, selected.ProfileName)
				if err := startSSMSessionFn(tmpConfigPath, selected.ProfileName, selected.Region, selected.InstanceID); err != nil {
					_ = removeFileFn(tmpConfigPath)
					return fmt.Errorf("ssm session failed: %w", err)
				}
				if len(selectedTargets) > 0 {
					scope := recentScope{
						AccountID:   selectedTargets[0].AccountID,
						AccountName: selectedTargets[0].AccountName,
						RoleName:    selectedTargets[0].RoleName,
						Region:      selected.Region,
					}
					inst := recentInstance{
						InstanceID:  selected.InstanceID,
						Region:      selected.Region,
						ProfileName: selected.ProfileName,
						DisplayLine: selected.DisplayLine,
					}
					if err := saveRecentTargets(opts.CacheDir, opts.Profile, scope, inst); err != nil {
						fmt.Printf("warning: failed to save recent target: %v\n", err)
					}
				}
				_ = removeFileFn(tmpConfigPath)
				return nil
			}

			_ = removeFileFn(tmpConfigPath)
			if backToRoles {
				continue
			}
		}
	}
}

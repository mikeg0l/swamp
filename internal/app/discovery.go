package app

import (
	"errors"
	"fmt"
	"strings"
)

func resolveSSORegion(cfg profileConfig) string {
	if strings.TrimSpace(cfg.SSORegion) != "" {
		return cfg.SSORegion
	}
	return "us-east-1"
}

func discoverAccounts(opts Options, ssoRegion, accessToken string) ([]ssoAccountsResponse, error) {
	fmt.Println("Discovering accessible AWS accounts...")
	accounts, err := listSSOAccountsCached(opts, ssoRegion, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to list SSO accounts: %w", err)
	}
	if len(accounts) == 0 {
		return nil, errors.New("no SSO accounts returned")
	}

	if opts.AccountFilter != "" {
		accounts = filterAccounts(accounts, opts.AccountFilter)
		if len(accounts) == 0 {
			return nil, fmt.Errorf("no accounts matched --account=%q", opts.AccountFilter)
		}
	}
	return accounts, nil
}

func discoverRoleTargets(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
	if len(accounts) == 0 {
		return nil, nil
	}

	fmt.Println("Discovering viable SSO roles in each account...")
	targets, err := buildRoleTargets(opts, ssoRegion, accessToken, accounts, opts.Workers)
	if err != nil {
		return nil, fmt.Errorf("failed while listing account roles: %w", err)
	}
	if len(targets) == 0 {
		return nil, errors.New("no account/role combinations were discovered")
	}

	if opts.RoleFilter != "" {
		targets = filterRoleTargets(targets, opts.RoleFilter)
		if len(targets) == 0 {
			return nil, fmt.Errorf("no roles matched --role=%q", opts.RoleFilter)
		}
	}
	return targets, nil
}

func discoverRegions(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
	if len(targets) == 0 {
		return nil, nil
	}

	discoveryRegion := cfg.Region
	if strings.TrimSpace(discoveryRegion) == "" {
		discoveryRegion = ssoRegion
	}
	if strings.TrimSpace(discoveryRegion) == "" {
		discoveryRegion = "us-east-1"
	}

	discoveryProfile := profileNames[targetKey(targets[0])]
	if strings.TrimSpace(discoveryProfile) == "" {
		return nil, fmt.Errorf("failed to map discovery profile for target %s/%s", targets[0].AccountID, targets[0].RoleName)
	}

	regions, err := resolveRegionsCached(opts, tmpConfigPath, discoveryProfile, discoveryRegion, opts.RegionsArg, opts.AllRegions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve regions: %w", err)
	}
	if len(regions) == 0 {
		return nil, errors.New("no regions to scan")
	}
	return regions, nil
}

func filterAccounts(accounts []ssoAccountsResponse, filter string) []ssoAccountsResponse {
	needle := strings.ToLower(strings.TrimSpace(filter))
	if needle == "" {
		return accounts
	}
	var out []ssoAccountsResponse
	for _, a := range accounts {
		if len(a.AccountList) == 0 {
			continue
		}
		acct := a.AccountList[0]
		if acct.AccountID == needle || strings.Contains(strings.ToLower(acct.AccountName), needle) {
			out = append(out, a)
		}
	}
	return out
}

func filterRoleTargets(targets []roleTarget, role string) []roleTarget {
	needle := strings.TrimSpace(role)
	if needle == "" {
		return targets
	}
	var out []roleTarget
	for _, t := range targets {
		if t.RoleName == needle {
			out = append(out, t)
		}
	}
	return out
}

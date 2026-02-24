package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	listSSOAccountsFetcher   = listSSOAccounts
	fetchRolesForAcctFetcher = fetchRolesForAccount
)

func ensureSSOLoginAndGetToken(profile, preferredStartURL string) (string, error) {
	// Fast path: if an unexpired token already exists, skip login.
	if tok, err := loadSSOAccessToken(preferredStartURL); err == nil {
		return tok, nil
	}

	login := exec.Command("aws", "sso", "login", "--profile", profile)
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return "", err
	}
	return loadSSOAccessToken(preferredStartURL)
}

func listSSOAccounts(profile, ssoRegion, accessToken string) ([]ssoAccountsResponse, error) {
	out, err := runAWSJSON("", profile, []string{
		"sso", "list-accounts",
		"--region", ssoRegion,
		"--access-token", accessToken,
	})
	if err != nil {
		return nil, err
	}
	var resp ssoAccountsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode list-accounts response: %w", err)
	}

	var outAccounts []ssoAccountsResponse
	for _, a := range resp.AccountList {
		outAccounts = append(outAccounts, ssoAccountsResponse{
			AccountList: []struct {
				AccountID    string `json:"accountId"`
				AccountName  string `json:"accountName"`
				EmailAddress string `json:"emailAddress"`
			}{
				{
					AccountID:    a.AccountID,
					AccountName:  a.AccountName,
					EmailAddress: a.EmailAddress,
				},
			},
		})
	}
	return outAccounts, nil
}

func listSSOAccountsCached(opts Options, ssoRegion, accessToken string) ([]ssoAccountsResponse, error) {
	key := cacheKeyAccounts(opts.Profile, ssoRegion)
	var cached []ssoAccountsResponse
	if opts.cacheStore != nil {
		status, age, err := opts.cacheStore.readJSON(opts.Profile, key, &cached)
		if err == nil {
			if status == cacheHitFresh {
				fmt.Printf("Using cached accounts (age=%s)\n", age.Round(time.Second))
				return cached, nil
			}
			if status == cacheHitStale && opts.cacheStore.shouldUseStale() {
				fmt.Printf("Using cached accounts (stale, age=%s), refreshing...\n", age.Round(time.Second))
				opts.cacheStore.refreshAsync(func() error {
					fresh, fetchErr := listSSOAccountsFetcher(opts.Profile, ssoRegion, accessToken)
					if fetchErr != nil {
						return fetchErr
					}
					return opts.cacheStore.writeJSON(opts.Profile, key, opts.CacheTTLAccounts, fresh)
				})
				return cached, nil
			}
		}
	}

	fresh, err := listSSOAccountsFetcher(opts.Profile, ssoRegion, accessToken)
	if err != nil {
		return nil, err
	}
	if opts.cacheStore != nil {
		_ = opts.cacheStore.writeJSON(opts.Profile, key, opts.CacheTTLAccounts, fresh)
	}
	return fresh, nil
}

func buildRoleTargets(opts Options, ssoRegion, accessToken string, accounts []ssoAccountsResponse, workers int) ([]roleTarget, error) {
	type acctJob struct {
		AccountID   string
		AccountName string
	}
	type acctResult struct {
		Targets []roleTarget
		Err     error
	}

	jobs := make(chan acctJob, workers*2)
	results := make(chan acctResult, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				out, err := listRolesForAccountCached(opts, ssoRegion, accessToken, j.AccountID, j.AccountName)
				if err != nil {
					results <- acctResult{Err: err}
					continue
				}
				results <- acctResult{Targets: out}
			}
		}()
	}

	go func() {
		for _, acctWrap := range accounts {
			if len(acctWrap.AccountList) == 0 {
				continue
			}
			acct := acctWrap.AccountList[0]
			jobs <- acctJob{
				AccountID:   acct.AccountID,
				AccountName: acct.AccountName,
			}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var targets []roleTarget
	var firstErr error
	for r := range results {
		if r.Err != nil && firstErr == nil {
			firstErr = r.Err
		}
		targets = append(targets, r.Targets...)
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return targets, nil
}

func listRolesForAccountCached(opts Options, ssoRegion, accessToken, accountID, accountName string) ([]roleTarget, error) {
	key := cacheKeyRoles(opts.Profile, ssoRegion, accountID)
	var cached []roleTarget
	if opts.cacheStore != nil {
		status, age, err := opts.cacheStore.readJSON(opts.Profile, key, &cached)
		if err == nil {
			if status == cacheHitFresh {
				return cached, nil
			}
			if status == cacheHitStale && opts.cacheStore.shouldUseStale() {
				fmt.Printf("Using cached roles for account %s (stale, age=%s), refreshing...\n", accountID, age.Round(time.Second))
				opts.cacheStore.refreshAsync(func() error {
					fresh, fetchErr := fetchRolesForAcctFetcher(opts.Profile, ssoRegion, accessToken, accountID, accountName)
					if fetchErr != nil {
						return fetchErr
					}
					return opts.cacheStore.writeJSON(opts.Profile, key, opts.CacheTTLRoles, fresh)
				})
				return cached, nil
			}
		}
	}

	fresh, err := fetchRolesForAcctFetcher(opts.Profile, ssoRegion, accessToken, accountID, accountName)
	if err != nil {
		return nil, err
	}
	if opts.cacheStore != nil {
		_ = opts.cacheStore.writeJSON(opts.Profile, key, opts.CacheTTLRoles, fresh)
	}
	return fresh, nil
}

func fetchRolesForAccount(profile, ssoRegion, accessToken, accountID, accountName string) ([]roleTarget, error) {
	rolesOut, err := runAWSJSON("", profile, []string{
		"sso", "list-account-roles",
		"--region", ssoRegion,
		"--access-token", accessToken,
		"--account-id", accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("account %s (%s): %w", accountID, accountName, err)
	}

	var rolesResp ssoRolesResponse
	if err := json.Unmarshal(rolesOut, &rolesResp); err != nil {
		return nil, fmt.Errorf("decode list-account-roles for account %s: %w", accountID, err)
	}

	var out []roleTarget
	for _, r := range rolesResp.RoleList {
		if strings.TrimSpace(r.RoleName) == "" {
			continue
		}
		out = append(out, roleTarget{
			AccountID:   accountID,
			AccountName: accountName,
			RoleName:    r.RoleName,
		})
	}
	return out, nil
}

func loadSSOAccessToken(preferredStartURL string) (string, error) {
	cacheDir := awsSSOCacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("read SSO cache dir %q: %w", cacheDir, err)
	}

	now := time.Now()
	normalizedPreferred := normalizeStartURL(preferredStartURL)
	var bestToken string
	var bestExpiry time.Time

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var tok ssoCacheToken
		if err := json.Unmarshal(data, &tok); err != nil {
			continue
		}
		if strings.TrimSpace(tok.AccessToken) == "" || strings.TrimSpace(tok.ExpiresAt) == "" {
			continue
		}

		expiresAt, err := parseSSOExpiry(tok.ExpiresAt)
		if err != nil || !expiresAt.After(now) {
			continue
		}

		if normalizedPreferred != "" && normalizeStartURL(tok.StartURL) != normalizedPreferred {
			continue
		}
		if bestToken == "" || expiresAt.After(bestExpiry) {
			bestToken = tok.AccessToken
			bestExpiry = expiresAt
		}
	}

	if bestToken == "" && normalizedPreferred != "" {
		// Fallback for profiles that resolve to a session but don't expose start URL in profile section.
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(cacheDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var tok ssoCacheToken
			if err := json.Unmarshal(data, &tok); err != nil {
				continue
			}
			if strings.TrimSpace(tok.AccessToken) == "" || strings.TrimSpace(tok.ExpiresAt) == "" {
				continue
			}
			expiresAt, err := parseSSOExpiry(tok.ExpiresAt)
			if err != nil || !expiresAt.After(now) {
				continue
			}
			if bestToken == "" || expiresAt.After(bestExpiry) {
				bestToken = tok.AccessToken
				bestExpiry = expiresAt
			}
		}
	}

	if bestToken == "" {
		return "", errors.New("no valid, unexpired SSO access token found in ~/.aws/sso/cache (run aws sso login)")
	}
	return bestToken, nil
}

func parseSSOExpiry(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05UTC",
		"2006-01-02T15:04:05Z0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported expiresAt format: %q", value)
}

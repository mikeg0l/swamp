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

func buildRoleTargets(profile, ssoRegion, accessToken string, accounts []ssoAccountsResponse, workers int) ([]roleTarget, error) {
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
				rolesOut, err := runAWSJSON("", profile, []string{
					"sso", "list-account-roles",
					"--region", ssoRegion,
					"--access-token", accessToken,
					"--account-id", j.AccountID,
				})
				if err != nil {
					results <- acctResult{
						Err: fmt.Errorf("account %s (%s): %w", j.AccountID, j.AccountName, err),
					}
					continue
				}

				var rolesResp ssoRolesResponse
				if err := json.Unmarshal(rolesOut, &rolesResp); err != nil {
					results <- acctResult{
						Err: fmt.Errorf("decode list-account-roles for account %s: %w", j.AccountID, err),
					}
					continue
				}

				var out []roleTarget
				for _, r := range rolesResp.RoleList {
					if strings.TrimSpace(r.RoleName) == "" {
						continue
					}
					out = append(out, roleTarget{
						AccountID:   j.AccountID,
						AccountName: j.AccountName,
						RoleName:    r.RoleName,
					})
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

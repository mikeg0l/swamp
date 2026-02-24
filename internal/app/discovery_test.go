package app

import (
	"strings"
	"testing"
)

func TestDiscoverRoleTargetsPreferredRoleFallback(t *testing.T) {
	origBuild := fetchRolesForAcctFetcher
	defer func() {
		fetchRolesForAcctFetcher = origBuild
	}()

	fetchRolesForAcctFetcher = func(profile, ssoRegion, accessToken, accountID, accountName string) ([]roleTarget, error) {
		return []roleTarget{
			{AccountID: accountID, AccountName: accountName, RoleName: "ReadOnly"},
		}, nil
	}

	opts := Options{
		Profile:           "p",
		Workers:           1,
		RoleFilter:        "Admin",
		RoleFromPreferred: true,
		CacheEnabled:      false,
	}
	accounts := []ssoAccountsResponse{testAccount("123", "acct")}
	targets, err := discoverRoleTargets(opts, accounts, "us-east-1", "token")
	if err != nil {
		t.Fatalf("discoverRoleTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].RoleName != "ReadOnly" {
		t.Fatalf("expected unfiltered targets fallback, got %+v", targets)
	}
}

func TestDiscoverRoleTargetsExplicitRoleNoMatchFails(t *testing.T) {
	origBuild := fetchRolesForAcctFetcher
	defer func() { fetchRolesForAcctFetcher = origBuild }()

	fetchRolesForAcctFetcher = func(profile, ssoRegion, accessToken, accountID, accountName string) ([]roleTarget, error) {
		return []roleTarget{
			{AccountID: accountID, AccountName: accountName, RoleName: "ReadOnly"},
		}, nil
	}

	opts := Options{
		Profile:      "p",
		Workers:      1,
		RoleFilter:   "Admin",
		CacheEnabled: false,
	}
	accounts := []ssoAccountsResponse{testAccount("123", "acct")}
	_, err := discoverRoleTargets(opts, accounts, "us-east-1", "token")
	if err == nil || !strings.Contains(err.Error(), "no roles matched") {
		t.Fatalf("expected no roles matched error, got %v", err)
	}
}

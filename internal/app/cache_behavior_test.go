package app

import (
	"testing"
	"time"
)

func newTestCacheOptions(t *testing.T, mode string) Options {
	t.Helper()
	opts := Options{
		Profile:           "test-profile",
		CacheEnabled:      true,
		CacheDir:          t.TempDir(),
		CacheMode:         mode,
		CacheTTLAccounts:  time.Minute,
		CacheTTLRoles:     time.Minute,
		CacheTTLRegions:   time.Minute,
		CacheTTLInstances: time.Minute,
	}
	opts.cacheStore = newCacheStore(opts)
	return opts
}

func testAccount(id, name string) ssoAccountsResponse {
	return ssoAccountsResponse{AccountList: []struct {
		AccountID    string `json:"accountId"`
		AccountName  string `json:"accountName"`
		EmailAddress string `json:"emailAddress"`
	}{
		{AccountID: id, AccountName: name, EmailAddress: "n/a"},
	}}
}

func TestListSSOAccountsCachedFreshHitSkipsFetcher(t *testing.T) {
	opts := newTestCacheOptions(t, "balanced")
	key := cacheKeyAccounts(opts.Profile, "us-east-1")
	cached := []ssoAccountsResponse{testAccount("111", "cached")}
	if err := opts.cacheStore.writeJSON(opts.Profile, key, time.Minute, cached); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	orig := listSSOAccountsFetcher
	defer func() { listSSOAccountsFetcher = orig }()
	calls := 0
	listSSOAccountsFetcher = func(profile, ssoRegion, accessToken string) ([]ssoAccountsResponse, error) {
		calls++
		return []ssoAccountsResponse{testAccount("222", "fresh")}, nil
	}

	got, err := listSSOAccountsCached(opts, "us-east-1", "token")
	if err != nil {
		t.Fatalf("listSSOAccountsCached failed: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected fetcher to be skipped, calls=%d", calls)
	}
	if got[0].AccountList[0].AccountID != "111" {
		t.Fatalf("expected cached account, got %+v", got)
	}
}

func TestListSSOAccountsCachedFreshModeBypassesCache(t *testing.T) {
	opts := newTestCacheOptions(t, "fresh")
	key := cacheKeyAccounts(opts.Profile, "us-east-1")
	if err := opts.cacheStore.writeJSON(opts.Profile, key, time.Minute, []ssoAccountsResponse{testAccount("111", "cached")}); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	orig := listSSOAccountsFetcher
	defer func() { listSSOAccountsFetcher = orig }()
	listSSOAccountsFetcher = func(profile, ssoRegion, accessToken string) ([]ssoAccountsResponse, error) {
		return []ssoAccountsResponse{testAccount("222", "fresh")}, nil
	}

	got, err := listSSOAccountsCached(opts, "us-east-1", "token")
	if err != nil {
		t.Fatalf("listSSOAccountsCached failed: %v", err)
	}
	if got[0].AccountList[0].AccountID != "222" {
		t.Fatalf("expected fresh account, got %+v", got)
	}
}

func TestListSSOAccountsCachedStaleReturnsImmediatelyAndRefreshes(t *testing.T) {
	opts := newTestCacheOptions(t, "balanced")
	key := cacheKeyAccounts(opts.Profile, "us-east-1")
	stale := []ssoAccountsResponse{testAccount("111", "stale")}
	if err := opts.cacheStore.writeJSON(opts.Profile, key, 10*time.Millisecond, stale); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	orig := listSSOAccountsFetcher
	defer func() { listSSOAccountsFetcher = orig }()
	called := make(chan struct{}, 1)
	listSSOAccountsFetcher = func(profile, ssoRegion, accessToken string) ([]ssoAccountsResponse, error) {
		called <- struct{}{}
		return []ssoAccountsResponse{testAccount("222", "fresh")}, nil
	}

	got, err := listSSOAccountsCached(opts, "us-east-1", "token")
	if err != nil {
		t.Fatalf("listSSOAccountsCached failed: %v", err)
	}
	if got[0].AccountList[0].AccountID != "111" {
		t.Fatalf("expected stale account returned immediately, got %+v", got)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async refresh to call fetcher")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		var refreshed []ssoAccountsResponse
		status, _, _ := opts.cacheStore.readJSON(opts.Profile, key, &refreshed)
		if status == cacheHitFresh && len(refreshed) > 0 && refreshed[0].AccountList[0].AccountID == "222" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected refreshed cache to contain fresh account, status=%v, refreshed=%+v", status, refreshed)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestQueryInstancesCachedFreshHitSkipsFetcher(t *testing.T) {
	opts := newTestCacheOptions(t, "balanced")
	target := roleTarget{AccountID: "123", RoleName: "Admin", AccountName: "acct"}
	key := cacheKeyInstances(opts.Profile, target.AccountID, target.RoleName, "us-east-1", true)
	cached := []instanceCandidate{{InstanceID: "i-cached", Region: "us-east-1", ProfileName: "p", DisplayLine: "cached"}}
	if err := opts.cacheStore.writeJSON(opts.Profile, key, time.Minute, cached); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	orig := queryInstancesFetcher
	defer func() { queryInstancesFetcher = orig }()
	calls := 0
	queryInstancesFetcher = func(tmpConfigPath string, target roleTarget, profileName, region string, runningOnly bool) ([]instanceCandidate, error) {
		calls++
		return []instanceCandidate{{InstanceID: "i-fresh"}}, nil
	}

	got, err := queryInstancesCached(opts, "", target, "p", "us-east-1", true)
	if err != nil {
		t.Fatalf("queryInstancesCached failed: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected fetcher to be skipped, calls=%d", calls)
	}
	if got[0].InstanceID != "i-cached" {
		t.Fatalf("expected cached instances, got %+v", got)
	}
}

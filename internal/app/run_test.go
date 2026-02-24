package app

import (
	"fmt"
	"testing"
)

func installRunTestSeams(t *testing.T) {
	t.Helper()

	origSelectAccountFn := selectAccountFn
	origDiscoverRoleTargetsFn := discoverRoleTargetsFn
	origSelectRoleTargetFn := selectRoleTargetFn
	origBuildTempAWSConfigFn := buildTempAWSConfigFn
	origDiscoverRegionsFn := discoverRegionsFn
	origSelectRegionFn := selectRegionFn
	origScanAllInstancesFn := scanAllInstancesFn
	origPickInstanceFn := pickInstanceFn
	origStartSSMSessionFn := startSSMSessionFn
	origRemoveFileFn := removeFileFn

	t.Cleanup(func() {
		selectAccountFn = origSelectAccountFn
		discoverRoleTargetsFn = origDiscoverRoleTargetsFn
		selectRoleTargetFn = origSelectRoleTargetFn
		buildTempAWSConfigFn = origBuildTempAWSConfigFn
		discoverRegionsFn = origDiscoverRegionsFn
		selectRegionFn = origSelectRegionFn
		scanAllInstancesFn = origScanAllInstancesFn
		pickInstanceFn = origPickInstanceFn
		startSSMSessionFn = origStartSSMSessionFn
		removeFileFn = origRemoveFileFn
	})

	selectAccountFn = func(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
		panic("unexpected selectAccountFn call")
	}
	discoverRoleTargetsFn = func(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
		panic("unexpected discoverRoleTargetsFn call")
	}
	selectRoleTargetFn = func(targets []roleTarget) (*roleTarget, bool, error) {
		panic("unexpected selectRoleTargetFn call")
	}
	buildTempAWSConfigFn = func(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
		panic("unexpected buildTempAWSConfigFn call")
	}
	discoverRegionsFn = func(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
		panic("unexpected discoverRegionsFn call")
	}
	selectRegionFn = func(regions []string) (string, bool, error) {
		panic("unexpected selectRegionFn call")
	}
	scanAllInstancesFn = func(opts Options, tmpConfigPath string, targets []roleTarget, profileNames map[string]string, regions []string, workers int, runningOnly bool) []instanceCandidate {
		panic("unexpected scanAllInstancesFn call")
	}
	pickInstanceFn = func(candidates []instanceCandidate) (*instanceCandidate, bool, error) {
		panic("unexpected pickInstanceFn call")
	}
	startSSMSessionFn = func(tmpConfigPath, profile, region, instanceID string) error {
		panic("unexpected startSSMSessionFn call")
	}
	removeFileFn = func(path string) error {
		panic("unexpected removeFileFn call")
	}
}

func TestRunInteractiveScopeInstanceBackReturnsToRegionPicker(t *testing.T) {
	installRunTestSeams(t)

	account := testAccount("111111111111", "acct")
	target := roleTarget{AccountID: "111111111111", AccountName: "acct", RoleName: "Admin"}
	mockConfigPath := "/tmp/mock-config.ini"
	profileNames := map[string]string{targetKey(target): "swamp-1"}
	candidate := instanceCandidate{DisplayLine: "line-1", ProfileName: "swamp-1", Region: "us-east-1", InstanceID: "i-123"}

	selectAccountFn = func(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
		return &account, nil
	}
	discoverRoleTargetsFn = func(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
		return []roleTarget{target}, nil
	}
	selectRoleTargetFn = func(targets []roleTarget) (*roleTarget, bool, error) {
		return &target, false, nil
	}
	buildTempAWSConfigFn = func(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
		return mockConfigPath, profileNames, nil
	}
	discoverRegionsFn = func(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
		return []string{"us-east-1"}, nil
	}
	regionCalls := 0
	selectRegionFn = func(regions []string) (string, bool, error) {
		regionCalls++
		return "us-east-1", false, nil
	}
	scanAllInstancesFn = func(opts Options, tmpConfigPath string, targets []roleTarget, profileNames map[string]string, regions []string, workers int, runningOnly bool) []instanceCandidate {
		return []instanceCandidate{candidate}
	}
	pickCalls := 0
	pickInstanceFn = func(candidates []instanceCandidate) (*instanceCandidate, bool, error) {
		pickCalls++
		if pickCalls == 1 {
			return nil, true, nil
		}
		return &candidate, false, nil
	}
	startCalls := 0
	startSSMSessionFn = func(tmpConfigPath, profile, region, instanceID string) error {
		startCalls++
		return nil
	}
	removeCalls := 0
	removeFileFn = func(path string) error {
		removeCalls++
		return nil
	}

	err := runInteractiveScope(Options{Workers: 1}, profileConfig{}, "us-east-1", "token", []ssoAccountsResponse{account})
	if err != nil {
		t.Fatalf("runInteractiveScope returned error: %v", err)
	}
	if regionCalls != 2 {
		t.Fatalf("expected region picker to be called twice, got %d", regionCalls)
	}
	if startCalls != 1 {
		t.Fatalf("expected one ssm start call, got %d", startCalls)
	}
	if removeCalls != 1 {
		t.Fatalf("expected one temp config cleanup, got %d", removeCalls)
	}
}

func TestRunInteractiveScopeRegionBackReturnsToRolePicker(t *testing.T) {
	installRunTestSeams(t)

	account := testAccount("111111111111", "acct")
	target := roleTarget{AccountID: "111111111111", AccountName: "acct", RoleName: "Admin"}
	mockConfigPath := "/tmp/mock-config.ini"
	profileNames := map[string]string{targetKey(target): "swamp-1"}

	selectAccountFn = func(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
		return &account, nil
	}
	discoverRoleTargetsFn = func(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
		return []roleTarget{target}, nil
	}
	roleCalls := 0
	selectRoleTargetFn = func(targets []roleTarget) (*roleTarget, bool, error) {
		roleCalls++
		if roleCalls == 1 {
			return &target, false, nil
		}
		return nil, false, nil
	}
	buildTempAWSConfigFn = func(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
		return mockConfigPath, profileNames, nil
	}
	discoverRegionsFn = func(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
		return []string{"us-east-1"}, nil
	}
	regionCalls := 0
	selectRegionFn = func(regions []string) (string, bool, error) {
		regionCalls++
		return "", true, nil
	}
	startCalls := 0
	startSSMSessionFn = func(tmpConfigPath, profile, region, instanceID string) error {
		startCalls++
		return nil
	}
	removeCalls := 0
	removeFileFn = func(path string) error {
		removeCalls++
		return nil
	}

	err := runInteractiveScope(Options{Workers: 1}, profileConfig{}, "us-east-1", "token", []ssoAccountsResponse{account})
	if err != nil {
		t.Fatalf("runInteractiveScope returned error: %v", err)
	}
	if roleCalls != 2 {
		t.Fatalf("expected role picker to be called twice, got %d", roleCalls)
	}
	if regionCalls != 1 {
		t.Fatalf("expected region picker to be called once, got %d", regionCalls)
	}
	if startCalls != 0 {
		t.Fatalf("expected no ssm start calls, got %d", startCalls)
	}
	if removeCalls != 1 {
		t.Fatalf("expected one temp config cleanup, got %d", removeCalls)
	}
}

func TestRunInteractiveScopeSuccessfulSelectionUsesMockConfigPath(t *testing.T) {
	installRunTestSeams(t)

	account := testAccount("111111111111", "acct")
	target := roleTarget{AccountID: "111111111111", AccountName: "acct", RoleName: "Admin"}
	mockConfigPath := "/tmp/mock-config.ini"
	profileNames := map[string]string{targetKey(target): "swamp-1"}
	selected := instanceCandidate{DisplayLine: "line-1", ProfileName: "swamp-1", Region: "us-east-1", InstanceID: "i-abc"}

	selectAccountFn = func(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
		return &account, nil
	}
	discoverRoleTargetsFn = func(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
		return []roleTarget{target}, nil
	}
	selectRoleTargetFn = func(targets []roleTarget) (*roleTarget, bool, error) {
		return &target, false, nil
	}
	buildTempAWSConfigFn = func(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
		return mockConfigPath, profileNames, nil
	}
	discoverRegionsFn = func(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
		return []string{"us-east-1"}, nil
	}
	selectRegionFn = func(regions []string) (string, bool, error) {
		return "us-east-1", false, nil
	}
	scanAllInstancesFn = func(opts Options, tmpConfigPath string, targets []roleTarget, profileNames map[string]string, regions []string, workers int, runningOnly bool) []instanceCandidate {
		return []instanceCandidate{selected}
	}
	pickInstanceFn = func(candidates []instanceCandidate) (*instanceCandidate, bool, error) {
		return &selected, false, nil
	}
	events := []string{}
	captured := struct {
		tmpConfigPath string
		profile       string
		region        string
		instanceID    string
	}{}
	startSSMSessionFn = func(tmpConfigPath, profile, region, instanceID string) error {
		events = append(events, "start")
		captured.tmpConfigPath = tmpConfigPath
		captured.profile = profile
		captured.region = region
		captured.instanceID = instanceID
		return nil
	}
	removeFileFn = func(path string) error {
		events = append(events, "remove")
		if path != mockConfigPath {
			return fmt.Errorf("unexpected remove path %q", path)
		}
		return nil
	}

	err := runInteractiveScope(Options{Workers: 1}, profileConfig{}, "us-east-1", "token", []ssoAccountsResponse{account})
	if err != nil {
		t.Fatalf("runInteractiveScope returned error: %v", err)
	}
	if captured.tmpConfigPath != mockConfigPath || captured.profile != selected.ProfileName || captured.region != selected.Region || captured.instanceID != selected.InstanceID {
		t.Fatalf("unexpected startSSMSession args: %+v", captured)
	}
	if len(events) != 2 || events[0] != "start" || events[1] != "remove" {
		t.Fatalf("expected start then cleanup, got %v", events)
	}
}

func TestRunInteractiveScopeNoInstanceSelectedCleansUpAndExits(t *testing.T) {
	installRunTestSeams(t)

	account := testAccount("111111111111", "acct")
	target := roleTarget{AccountID: "111111111111", AccountName: "acct", RoleName: "Admin"}
	mockConfigPath := "/tmp/mock-config.ini"
	profileNames := map[string]string{targetKey(target): "swamp-1"}
	candidate := instanceCandidate{DisplayLine: "line-1", ProfileName: "swamp-1", Region: "us-east-1", InstanceID: "i-123"}

	selectAccountFn = func(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
		return &account, nil
	}
	discoverRoleTargetsFn = func(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
		return []roleTarget{target}, nil
	}
	selectRoleTargetFn = func(targets []roleTarget) (*roleTarget, bool, error) {
		return &target, false, nil
	}
	buildTempAWSConfigFn = func(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
		return mockConfigPath, profileNames, nil
	}
	discoverRegionsFn = func(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
		return []string{"us-east-1"}, nil
	}
	selectRegionFn = func(regions []string) (string, bool, error) {
		return "us-east-1", false, nil
	}
	scanAllInstancesFn = func(opts Options, tmpConfigPath string, targets []roleTarget, profileNames map[string]string, regions []string, workers int, runningOnly bool) []instanceCandidate {
		return []instanceCandidate{candidate}
	}
	pickInstanceFn = func(candidates []instanceCandidate) (*instanceCandidate, bool, error) {
		return nil, false, nil
	}
	startCalls := 0
	startSSMSessionFn = func(tmpConfigPath, profile, region, instanceID string) error {
		startCalls++
		return nil
	}
	removeCalls := 0
	removeFileFn = func(path string) error {
		removeCalls++
		return nil
	}

	err := runInteractiveScope(Options{Workers: 1}, profileConfig{}, "us-east-1", "token", []ssoAccountsResponse{account})
	if err != nil {
		t.Fatalf("runInteractiveScope returned error: %v", err)
	}
	if startCalls != 0 {
		t.Fatalf("expected no ssm start calls, got %d", startCalls)
	}
	if removeCalls != 1 {
		t.Fatalf("expected one temp config cleanup, got %d", removeCalls)
	}
}

func TestRunInteractiveScopeNoInstancesLoopsToRegionSelection(t *testing.T) {
	installRunTestSeams(t)

	account := testAccount("111111111111", "acct")
	target := roleTarget{AccountID: "111111111111", AccountName: "acct", RoleName: "Admin"}
	mockConfigPath := "/tmp/mock-config.ini"
	profileNames := map[string]string{targetKey(target): "swamp-1"}
	selected := instanceCandidate{DisplayLine: "line-1", ProfileName: "swamp-1", Region: "us-east-1", InstanceID: "i-xyz"}

	selectAccountFn = func(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
		return &account, nil
	}
	discoverRoleTargetsFn = func(opts Options, accounts []ssoAccountsResponse, ssoRegion, accessToken string) ([]roleTarget, error) {
		return []roleTarget{target}, nil
	}
	selectRoleTargetFn = func(targets []roleTarget) (*roleTarget, bool, error) {
		return &target, false, nil
	}
	buildTempAWSConfigFn = func(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
		return mockConfigPath, profileNames, nil
	}
	discoverRegionsFn = func(opts Options, cfg profileConfig, targets []roleTarget, tmpConfigPath string, profileNames map[string]string, ssoRegion string) ([]string, error) {
		return []string{"us-east-1"}, nil
	}
	regionCalls := 0
	selectRegionFn = func(regions []string) (string, bool, error) {
		regionCalls++
		return "us-east-1", false, nil
	}
	scanCalls := 0
	scanAllInstancesFn = func(opts Options, tmpConfigPath string, targets []roleTarget, profileNames map[string]string, regions []string, workers int, runningOnly bool) []instanceCandidate {
		scanCalls++
		if scanCalls == 1 {
			return nil
		}
		return []instanceCandidate{selected}
	}
	pickInstanceFn = func(candidates []instanceCandidate) (*instanceCandidate, bool, error) {
		return &selected, false, nil
	}
	startCalls := 0
	startSSMSessionFn = func(tmpConfigPath, profile, region, instanceID string) error {
		startCalls++
		return nil
	}
	removeCalls := 0
	removeFileFn = func(path string) error {
		removeCalls++
		return nil
	}

	err := runInteractiveScope(Options{Workers: 1}, profileConfig{}, "us-east-1", "token", []ssoAccountsResponse{account})
	if err != nil {
		t.Fatalf("runInteractiveScope returned error: %v", err)
	}
	if regionCalls != 2 {
		t.Fatalf("expected region picker to be called twice, got %d", regionCalls)
	}
	if scanCalls != 2 {
		t.Fatalf("expected scan to be called twice, got %d", scanCalls)
	}
	if startCalls != 1 {
		t.Fatalf("expected one ssm start call, got %d", startCalls)
	}
	if removeCalls != 1 {
		t.Fatalf("expected one temp config cleanup, got %d", removeCalls)
	}
}

package app

import (
	"testing"
)

func TestRecentTargetsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	scope := recentScope{
		AccountID:   "123456789012",
		AccountName: "acct",
		RoleName:    "Admin",
		Region:      "us-east-1",
	}
	inst := recentInstance{
		InstanceID:  "i-abc",
		Region:      "us-east-1",
		ProfileName: "swamp-1",
		DisplayLine: "line",
	}
	if err := saveRecentTargets(dir, "my-profile", scope, inst); err != nil {
		t.Fatalf("saveRecentTargets failed: %v", err)
	}

	got, err := loadRecentTargets(dir)
	if err != nil {
		t.Fatalf("loadRecentTargets failed: %v", err)
	}
	gotScope, ok := got.getLastScope("my-profile")
	if !ok {
		t.Fatal("expected scope to exist")
	}
	if gotScope.AccountID != scope.AccountID || gotScope.RoleName != scope.RoleName {
		t.Fatalf("unexpected scope: %+v", gotScope)
	}
	_, gotInst, ok := got.getLastInstance("my-profile")
	if !ok {
		t.Fatal("expected instance to exist")
	}
	if gotInst.InstanceID != inst.InstanceID || gotInst.Region != inst.Region {
		t.Fatalf("unexpected instance: %+v", gotInst)
	}
}

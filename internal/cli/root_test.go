package cli

import "testing"

func TestRootCommandCacheDefaultTTLs(t *testing.T) {
	cmd := newRootCmd()
	accounts := cmd.Flags().Lookup("cache-ttl-accounts")
	roles := cmd.Flags().Lookup("cache-ttl-roles")
	if accounts == nil || roles == nil {
		t.Fatal("expected cache TTL flags to exist")
	}
	if accounts.DefValue != "6h0m0s" {
		t.Fatalf("expected accounts default 6h0m0s, got %s", accounts.DefValue)
	}
	if roles.DefValue != "6h0m0s" {
		t.Fatalf("expected roles default 6h0m0s, got %s", roles.DefValue)
	}
}

func TestRootCommandNewShortFlags(t *testing.T) {
	cmd := newRootCmd()
	last := cmd.Flags().ShorthandLookup("l")
	resume := cmd.Flags().ShorthandLookup("u")
	if last == nil || last.Name != "last" {
		t.Fatalf("expected -l shorthand for --last")
	}
	if resume == nil || resume.Name != "resume" {
		t.Fatalf("expected -u shorthand for --resume")
	}
}

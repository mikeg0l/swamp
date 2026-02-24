package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"swamp/internal/app"
)

func Execute() error {
	return newRootCmd().Execute()
}

func ExecuteWithVersion(version string) error {
	return newRootCmdWithVersion(version).Execute()
}

func newRootCmd() *cobra.Command {
	return newRootCmdWithVersion("")
}

func newRootCmdWithVersion(version string) *cobra.Command {
	var opts app.Options

	cmd := &cobra.Command{
		Use:           "swamp",
		Short:         "Discover EC2 instances across SSO scope and connect via SSM",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = strings.TrimSpace(opts.Profile)
			opts.AccountFilter = strings.TrimSpace(opts.AccountFilter)
			opts.RoleFilter = strings.TrimSpace(opts.RoleFilter)
			opts.RegionsArg = strings.TrimSpace(opts.RegionsArg)
			opts.ConfigPath = strings.TrimSpace(opts.ConfigPath)
			opts.FlagSet = map[string]bool{
				"profile":                cmd.Flags().Changed("profile"),
				"workers":                cmd.Flags().Changed("workers"),
				"account":                cmd.Flags().Changed("account"),
				"role":                   cmd.Flags().Changed("role"),
				"regions":                cmd.Flags().Changed("regions"),
				"all-regions":            cmd.Flags().Changed("all-regions"),
				"include-stopped":        cmd.Flags().Changed("include-stopped"),
				"cache":                  cmd.Flags().Changed("cache"),
				"cache-dir":              cmd.Flags().Changed("cache-dir"),
				"cache-mode":             cmd.Flags().Changed("cache-mode"),
				"cache-clear":            cmd.Flags().Changed("cache-clear"),
				"cache-ttl-accounts":     cmd.Flags().Changed("cache-ttl-accounts"),
				"cache-ttl-roles":        cmd.Flags().Changed("cache-ttl-roles"),
				"cache-ttl-regions":      cmd.Flags().Changed("cache-ttl-regions"),
				"cache-ttl-instances":    cmd.Flags().Changed("cache-ttl-instances"),
				"resume":                 cmd.Flags().Changed("resume"),
				"last":                   cmd.Flags().Changed("last"),
				"no-auto-select":         cmd.Flags().Changed("no-auto-select"),
				"config":                 cmd.Flags().Changed("config"),
				"write-config-example":   cmd.Flags().Changed("write-config-example"),
				"print-effective-config": cmd.Flags().Changed("print-effective-config"),
			}
			return app.Run(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Profile, "profile", "p", "", "AWS SSO profile name to bootstrap discovery (required)")
	cmd.Flags().IntVarP(&opts.Workers, "workers", "w", 12, "Number of concurrent workers for account/role/region scanning")
	cmd.Flags().StringVarP(&opts.AccountFilter, "account", "a", "", "Filter to a specific account ID or account-name substring")
	cmd.Flags().StringVarP(&opts.RoleFilter, "role", "r", "", "Filter to a specific role name")
	cmd.Flags().StringVarP(&opts.RegionsArg, "regions", "R", "", "Comma-separated regions to scan (default: discover all enabled regions)")
	cmd.Flags().BoolVarP(&opts.AllRegions, "all-regions", "A", false, "Include all regions, even those not enabled in the account")
	cmd.Flags().BoolVarP(&opts.IncludeStopped, "include-stopped", "s", false, "Include non-running instances in selection")
	cmd.Flags().BoolVarP(&opts.Resume, "resume", "u", false, "Resume with the last successful account/role/region scope")
	cmd.Flags().BoolVarP(&opts.Last, "last", "l", false, "Reconnect directly to the last successful instance")
	cmd.Flags().BoolVar(&opts.NoAutoSelect, "no-auto-select", false, "Disable auto-selection when only one option is available")
	cmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", "", "Path to swamp config file (default: ~/.config/swamp/config.yaml)")
	cmd.Flags().BoolVar(&opts.WriteConfigExample, "write-config-example", false, "Write an example config file and exit")
	cmd.Flags().BoolVar(&opts.PrintEffectiveConfig, "print-effective-config", false, "Print effective runtime settings and exit")
	cmd.Flags().BoolVar(&opts.CacheEnabled, "cache", true, "Enable local discovery cache")
	cmd.Flags().StringVar(&opts.CacheDir, "cache-dir", app.DefaultCacheDirForCLI(), "Directory for local cache files")
	cmd.Flags().DurationVar(&opts.CacheTTLAccounts, "cache-ttl-accounts", 6*time.Hour, "TTL for SSO account discovery cache")
	cmd.Flags().DurationVar(&opts.CacheTTLRoles, "cache-ttl-roles", 6*time.Hour, "TTL for SSO role discovery cache")
	cmd.Flags().DurationVar(&opts.CacheTTLRegions, "cache-ttl-regions", 24*time.Hour, "TTL for region discovery cache")
	cmd.Flags().DurationVar(&opts.CacheTTLInstances, "cache-ttl-instances", 60*time.Second, "TTL for instance discovery cache")
	cmd.Flags().StringVar(&opts.CacheMode, "cache-mode", "balanced", "Cache mode: balanced, fresh, speed")
	cmd.Flags().BoolVar(&opts.CacheClear, "cache-clear", false, "Clear cache directory before discovery")

	return cmd
}

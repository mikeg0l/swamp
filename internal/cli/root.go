package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"swamp/internal/app"
)

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	var opts app.Options

	cmd := &cobra.Command{
		Use:          "swamp",
		Short:        "Discover EC2 instances across SSO scope and connect via SSM",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = strings.TrimSpace(opts.Profile)
			opts.AccountFilter = strings.TrimSpace(opts.AccountFilter)
			opts.RoleFilter = strings.TrimSpace(opts.RoleFilter)
			opts.RegionsArg = strings.TrimSpace(opts.RegionsArg)
			return app.Run(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Profile, "profile", "", "AWS SSO profile name to bootstrap discovery (required)")
	cmd.Flags().IntVar(&opts.Workers, "workers", 12, "Number of concurrent workers for account/role/region scanning")
	cmd.Flags().StringVar(&opts.AccountFilter, "account", "", "Filter to a specific account ID or account-name substring")
	cmd.Flags().StringVar(&opts.RoleFilter, "role", "", "Filter to a specific role name")
	cmd.Flags().StringVar(&opts.RegionsArg, "regions", "", "Comma-separated regions to scan (default: discover all enabled regions)")
	cmd.Flags().BoolVar(&opts.AllRegions, "all-regions", false, "Include all regions, even those not enabled in the account")
	cmd.Flags().BoolVar(&opts.InteractiveScope, "interactive-scope", true, "Interactively pick account, role, and region with fzf before listing instances (default: true; disable with --interactive-scope=false)")
	cmd.Flags().BoolVar(&opts.IncludeStopped, "include-stopped", false, "Include non-running instances in selection")
	_ = cmd.MarkFlagRequired("profile")

	return cmd
}

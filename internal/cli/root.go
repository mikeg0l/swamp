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
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Profile = strings.TrimSpace(opts.Profile)
			opts.AccountFilter = strings.TrimSpace(opts.AccountFilter)
			opts.RoleFilter = strings.TrimSpace(opts.RoleFilter)
			opts.RegionsArg = strings.TrimSpace(opts.RegionsArg)
			return app.Run(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Profile, "profile", "p", "", "AWS SSO profile name to bootstrap discovery (required)")
	cmd.Flags().IntVarP(&opts.Workers, "workers", "w", 12, "Number of concurrent workers for account/role/region scanning")
	cmd.Flags().StringVarP(&opts.AccountFilter, "account", "a", "", "Filter to a specific account ID or account-name substring")
	cmd.Flags().StringVarP(&opts.RoleFilter, "role", "r", "", "Filter to a specific role name")
	cmd.Flags().StringVarP(&opts.RegionsArg, "regions", "R", "", "Comma-separated regions to scan (default: discover all enabled regions)")
	cmd.Flags().BoolVarP(&opts.AllRegions, "all-regions", "A", false, "Include all regions, even those not enabled in the account")
	cmd.Flags().BoolVarP(&opts.InteractiveScope, "interactive-scope", "i", true, "Interactively pick account, role, and region with fzf before listing instances (default: true; disable with --interactive-scope=false)")
	cmd.Flags().BoolVarP(&opts.IncludeStopped, "include-stopped", "s", false, "Include non-running instances in selection")
	_ = cmd.MarkFlagRequired("profile")

	return cmd
}

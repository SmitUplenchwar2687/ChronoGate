package chronogatecli

import (
	chronocli "github.com/SmitUplenchwar2687/Chrono/pkg/cli"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the ChronoGate CLI.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "chronogate",
		Short:        "ChronoGate API and tooling powered by the Chrono SDK",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadServeConfig(cmd)
			if err != nil {
				return err
			}
			return runServe(cmd.Context(), cfg, false, ":9090", cmd.OutOrStdout())
		},
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newReplayCmd())

	sdk := chronocli.NewRootCmd()
	sdk.Use = "chrono-sdk"
	sdk.Short = "Run the Chrono SDK CLI from ChronoGate"
	root.AddCommand(sdk)

	return root
}

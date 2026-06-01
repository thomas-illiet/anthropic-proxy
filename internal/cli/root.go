package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is injected by release builds through -ldflags.
var version = "dev"

// Execute runs the Cobra command tree and owns process exit behavior.
func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCommand builds the program CLI without starting the server implicitly.
func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "anthropic-proxy",
		Short:             "Expose an Anthropic-compatible API over an OpenAI-compatible upstream",
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Args:              cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newServeCommand())
	cmd.AddCommand(newVersionCommand())
	return cmd
}

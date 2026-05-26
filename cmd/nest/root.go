package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/mhemeryck/nest/internal/nest"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	var opts nest.Options

	cmd := &cobra.Command{
		Use:          "nest <unit_id>",
		Short:        "Run the nest controller",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runRootCommand(&opts),
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "Path to config file")
	cmd.Flags().BoolVar(&opts.ValidateOnly, "validate", false, "Validate config and exit")

	return cmd
}

func runRootCommand(opts *nest.Options) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, args []string) error {
		runOpts := *opts
		runOpts.UnitID = args[0]

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		return nest.Run(ctx, runOpts)
	}
}

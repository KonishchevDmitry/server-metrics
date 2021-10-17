package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/KonishchevDmitry/server-metrics/internal/cgroups/collector"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/server"
)

func run() error {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [flags]", os.Args[0]),
		Short: "Server metrics daemon",
		Args:  cobra.NoArgs,

		SilenceUsage:          true,
		SilenceErrors:         true,
		DisableFlagsInUseLine: true,

		Run: func(cmd *cobra.Command, args []string) {
			if err := execute(cmd); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: %s.\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().Bool("devel", false, "print discovered metrics and exit")

	return cmd.Execute()
}

func execute(cmd *cobra.Command) error {
	flags := cmd.Flags()

	develMode, err := flags.GetBool("devel")
	if err != nil {
		return err
	}

	logger, err := logging.Configure(develMode)
	if err != nil {
		return err
	}
	defer func() {
		_ = logger.Sync() // Always fails to sync stderr
	}()

	collector := collector.NewCollector()

	if develMode {
		logger.Info("Running in test mode.")
		ctx := logging.WithLogger(context.Background(), logger)
		collector.Collect(ctx)
		return nil
	}

	return server.Start(logger, collector.Collect)
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Command line arguments parsing error: %s.\n", err)
		os.Exit(1)
	}
}

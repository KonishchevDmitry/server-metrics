package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	cgroupclassifier "github.com/KonishchevDmitry/server-metrics/internal/cgroups/classifier"
	cgroupscollector "github.com/KonishchevDmitry/server-metrics/internal/cgroups/collector"
	"github.com/KonishchevDmitry/server-metrics/internal/docker"
	"github.com/KonishchevDmitry/server-metrics/internal/logging"
	"github.com/KonishchevDmitry/server-metrics/internal/metrics"
	"github.com/KonishchevDmitry/server-metrics/internal/network"
	"github.com/KonishchevDmitry/server-metrics/internal/server"
	"github.com/KonishchevDmitry/server-metrics/internal/users"
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
	ctx := logging.WithLogger(context.Background(), logger)

	dockerResolver := docker.NewResolver()
	defer func() {
		if err := dockerResolver.Close(); err != nil {
			logging.L(ctx).Errorf("Failed to close Docker resolver: %s.", err)
		}
	}()

	cgroupClassifier := cgroupclassifier.New(users.NewResolver(), dockerResolver)
	cgroupsCollector := cgroupscollector.NewCollector(logger, cgroupClassifier)

	networkCollector, err := network.NewCollector(logger, develMode)
	if err != nil {
		return err
	}
	defer networkCollector.Close(ctx)

	registry := prometheus.NewRegistry()
	if develMode {
		registry = prometheus.NewPedanticRegistry()
	}
	if err := prometheus.WrapRegistererWithPrefix("services_", registry).Register(cgroupsCollector); err != nil {
		return err
	}
	if err := prometheus.WrapRegistererWithPrefix("network_", registry).Register(networkCollector); err != nil {
		return err
	}
	if err := prometheus.WrapRegistererWithPrefix(metrics.Namespace+"_", prometheus.DefaultRegisterer).Register(registry); err != nil {
		return err
	}

	collect := func() {
		metrics := make(chan prometheus.Metric)

		go func() {
			defer close(metrics)
			cgroupsCollector.Collect(metrics)
			networkCollector.Collect(metrics)
		}()

		for range metrics {
		}
	}

	if develMode {
		logger.Info("Running in devel mode.")

		collect()
		logger.Info("Sleeping...")
		time.Sleep(5 * time.Second)
		collect()

		return nil
	}

	return server.Start(ctx)
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Command line arguments parsing error: %s.\n", err)
		os.Exit(1)
	}
}

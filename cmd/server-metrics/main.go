package main

import (
	"context"
	"fmt"
	"os"
	"time"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"

	cgroupclassifier "github.com/KonishchevDmitry/server-metrics/internal/cgroups/classifier"
	cgroupscollector "github.com/KonishchevDmitry/server-metrics/internal/cgroups/collector"
	"github.com/KonishchevDmitry/server-metrics/internal/docker"
	"github.com/KonishchevDmitry/server-metrics/internal/kernel"
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

	flags := cmd.Flags()
	flags.Bool("devel", false, "print discovered metrics and exit")
	flags.String("bind-address", "127.0.0.1:9101", "address to bind to")
	flags.Bool("no-network-collector", false, "disable network collector")

	return cmd.Execute()
}

func execute(cmd *cobra.Command) error {
	flags := cmd.Flags()

	develMode, err := flags.GetBool("devel")
	if err != nil {
		return err
	}

	bindAddress, err := flags.GetString("bind-address")
	if err != nil {
		return err
	}

	withoutNetworkCollector, err := flags.GetBool("no-network-collector")
	if err != nil {
		return err
	}

	logLevel := zapcore.InfoLevel
	if develMode {
		logLevel = zapcore.DebugLevel
	}

	logger, err := logging.Configure(logging.Config{
		Daemon:           !develMode,
		Level:            logLevel,
		ShowLevel:        develMode,
		SyslogIdentifier: "server-metrics",
		OnError:          metrics.ErrorsMetric.Inc,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to flush the logger: %s.\n", err)
		}
	}()
	ctx := logging.WithLogger(context.Background(), logger)

	registry := prometheus.DefaultRegisterer
	gatherer := prometheus.DefaultGatherer

	kernelCollector, err := kernel.NewCollector(ctx)
	if err != nil {
		return err
	}
	defer kernelCollector.Close(ctx)

	if err := registry.Register(kernelCollector); err != nil {
		return err
	}

	dockerResolver := docker.NewResolver()
	defer func() {
		if err := dockerResolver.Close(); err != nil {
			logging.L(ctx).Errorf("Failed to close Docker resolver: %s.", err)
		}
	}()

	cgroupClassifier := cgroupclassifier.New(users.NewResolver(), dockerResolver)

	cgroupsCollector := cgroupscollector.NewCollector(logger, cgroupClassifier)
	if err := registry.Register(cgroupsCollector); err != nil {
		return err
	}

	if !withoutNetworkCollector {
		networkCollector, err := network.NewCollector(logger, develMode)
		if err != nil {
			return err
		}
		defer networkCollector.Close(ctx)

		if err := registry.Register(networkCollector); err != nil {
			return err
		}
	}

	if develMode {
		logger.Info("Running in devel mode.")

		if _, err := gatherer.Gather(); err != nil {
			return err
		}

		logger.Info("Sleeping...")
		time.Sleep(5 * time.Second)

		if _, err := gatherer.Gather(); err != nil {
			return err
		}

		return nil
	}

	return server.Start(ctx, bindAddress)
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Command line arguments parsing error: %s.\n", err)
		os.Exit(1)
	}
}

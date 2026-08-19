package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AndreZiviani/aws-health-exporter/exporter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// version is injected at build time via -ldflags.
var version = "dev"

func main() {
	cmd := &cli.Command{
		Name:    "aws-health-exporter",
		Usage:   "Export AWS Health events as Prometheus metrics and Slack notifications",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen-address", Aliases: []string{"l"}, Usage: "The address to listen on for HTTP requests", Value: ":8080"},
			&cli.StringFlag{Name: "metrics-path", Aliases: []string{"m"}, Usage: "Metrics endpoint path", Value: "/metrics"},
			&cli.StringSliceFlag{Name: "regions", Aliases: []string{"r"}, Usage: "Comma separated list of AWS regions to monitor (default: all regions)"},
			&cli.StringFlag{Name: "log-level", Aliases: []string{"v"}, Usage: "Log level (debug, info, warn, error)", Value: "info"},
			&cli.StringFlag{Name: "log-format", Usage: "Log format (text or json)", Value: "text"},
			&cli.StringFlag{Name: "slack-token", Usage: "Slack token", Sources: cli.EnvVars("SLACK_TOKEN")},
			&cli.StringFlag{Name: "slack-channel", Usage: "Slack channel id", Sources: cli.EnvVars("SLACK_CHANNEL")},
			&cli.StringFlag{Name: "assume-role", Usage: "Assume another AWS IAM role", Sources: cli.EnvVars("ASSUME_ROLE")},
			&cli.StringSliceFlag{Name: "ignore-events", Usage: "Comma separated list of events to be ignored on all resources"},
			&cli.StringSliceFlag{Name: "ignore-resources", Usage: "Comma separated list of resources to be ignored on all events, format is dependant on resource type (some are ARN others are Name, check AWS docs)"},
			&cli.StringSliceFlag{Name: "ignore-resource-event", Usage: "Comma separated list of events to be ignored on a specific resource (format: <event name>:<resource identifier>)"},
			&cli.BoolFlag{Name: "log-events", Usage: "Log AWS Health events on console"},

			&cli.DurationFlag{Name: "time-shift", Usage: "[INTERNAL] Apply a time delta to event filter instead of looking at previous scrape", Hidden: true},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	setupLogging(cmd.String("log-level"), cmd.String("log-format"))

	slog.Info("starting aws-health-exporter",
		"version", version,
		"log_level", cmd.String("log-level"),
		"log_events", cmd.Bool("log-events"),
	)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	provider, err := newMeterProvider()
	if err != nil {
		return fmt.Errorf("initializing meter provider: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shut down meter provider", "error", err)
		}
	}()

	opts := exporter.Options{
		Regions:             cmd.StringSlice("regions"),
		AssumeRole:          cmd.String("assume-role"),
		SlackToken:          cmd.String("slack-token"),
		SlackChannel:        cmd.String("slack-channel"),
		IgnoreEvents:        cmd.StringSlice("ignore-events"),
		IgnoreResources:     cmd.StringSlice("ignore-resources"),
		IgnoreResourceEvent: cmd.StringSlice("ignore-resource-event"),
		LogEvents:           cmd.Bool("log-events"),
		TimeShift:           cmd.Duration("time-shift"),
	}

	if _, err := exporter.New(ctx, otel.Meter("aws-health-exporter"), opts); err != nil {
		return fmt.Errorf("initializing exporter: %w", err)
	}

	return serveMetrics(ctx, cmd.String("listen-address"), cmd.String("metrics-path"))
}

func setupLogging(level, format string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}

	handlerOpts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}

	slog.SetDefault(slog.New(handler))
}

func newMeterProvider() (*metric.MeterProvider, error) {
	promExporter, err := otelprom.New(otelprom.WithNamespace("aws_health"))
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("aws-health-exporter"),
			semconv.ServiceVersion(version),
		))
	if err != nil {
		return nil, err
	}

	provider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(promExporter))
	otel.SetMeterProvider(provider)

	if err := runtime.Start(); err != nil {
		return nil, err
	}

	return provider, nil
}

func serveMetrics(ctx context.Context, addr, metricsPath string) error {
	slog.Info("starting metrics http endpoint", "address", addr, "path", metricsPath)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<html>
			<head><title>AWS Health Exporter</title></head>
			<body>
			<h1>AWS Health Exporter</h1>
			<p><a href=%q>Metrics</a></p>
			</body>
			</html>`, metricsPath)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}

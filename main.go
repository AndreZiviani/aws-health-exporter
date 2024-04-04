package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/AndreZiviani/aws-health-exporter/exporter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	flags := []cli.Flag{
		&cli.StringFlag{Name: "listen-address", Aliases: []string{"l"}, Usage: "The address to listen on for HTTP requests.", Value: ":8080"},
		&cli.StringFlag{Name: "metrics-path", Aliases: []string{"m"}, Usage: "Metrics endpoint path", Value: "/metrics"},
		&cli.StringFlag{Name: "regions", Aliases: []string{"r"}, Usage: "Comma separated list of AWS regions to monitor", Value: "all-regions"},
		&cli.StringFlag{Name: "log-level", Aliases: []string{"v"}, Usage: "Log level", Value: "info"},
		&cli.StringFlag{Name: "slack-token", Usage: "Slack token", EnvVars: []string{"SLACK_TOKEN"}},
		&cli.StringFlag{Name: "slack-channel", Usage: "Slack channel id", EnvVars: []string{"SLACK_CHANNEL"}},
		&cli.StringFlag{Name: "assume-role", Usage: "Assume another AWS IAM role", EnvVars: []string{"ASSUME_ROLE"}},
		&cli.StringFlag{Name: "ignore-events", Usage: "Comma separated list of events to be ignored on all resources"},
		&cli.StringFlag{Name: "ignore-resources", Usage: "Comma separated list of resources to be ignored on all events, format is dependant on resource type (some are ARN others are Name, check AWS docs)"},
		&cli.StringFlag{Name: "ignore-resource-event", Usage: "Comma separated list of events to be ignored on a specific resource (format: <event name>:<resource identifier>)"},

		&cli.DurationFlag{Name: "time-shift", Usage: "[INTERNAL] Apply a time delta to event filter instead of looking at previous scrape", Hidden: true, Value: 0 * time.Second},
	}

	app := &cli.App{
		Flags: flags,
		Name:  "aws-health-exporter",
		Action: func(c *cli.Context) error {
			parsedLevel, err := log.ParseLevel(c.String("log-level"))
			if err != nil {
				log.WithError(err).Warnf("Couldn't parse log level, using default: %s", log.GetLevel())
			} else {
				log.SetLevel(parsedLevel)
				log.Debugf("Set log level to %s", parsedLevel)
			}

			log.Infof("Starting AWS Health Exporter. [log-level=%s]", c.String("log-level"))

			ctx := context.Background()

			provider, err := newMeter()
			if err != nil {
				log.Fatal(err)
			}
			defer provider.Shutdown(ctx)

			_, err = exporter.NewMetrics(ctx, otel.Meter("aws-health-exporter"), c)
			if err != nil {
				log.Fatal(err)
			}

			serveMetrics(c)

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return
}

func newMeter() (*metric.MeterProvider, error) {
	promExporter, err := prometheus.New(prometheus.WithNamespace("aws_health"))
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("aws-health-exporter"),
			//semconv.ServiceVersion("0.1.0"),
		))
	if err != nil {
		return nil, err
	}

	provider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(promExporter))
	otel.SetMeterProvider(provider)
	runtime.Start()

	return provider, nil
}

func serveMetrics(c *cli.Context) {
	log.Infof("Starting metric http endpoint [address=%s, path=%s, regions=%s]", c.String("listen-address"), c.String("metrics-path"), c.String("regions"))
	http.Handle(c.String("metrics-path"), promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html>
				<head><title>AWS Health Exporter</title></head>
				<body>
				<h1>AWS Health Exporter</h1>
				<p><a href="` + c.String("metrics-path") + `">Metrics</a></p>
				</body>
				</html>
			`))

	})
	log.Fatal(http.ListenAndServe(c.String("listen-address"), nil))
}

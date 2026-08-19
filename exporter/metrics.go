package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"
	_ "time/tzdata"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// New creates the exporter and registers the AWS Health event gauge on the
// provided meter. Events are fetched lazily, on every scrape.
func New(ctx context.Context, meter metric.Meter, opts Options) (*Metrics, error) {
	m, err := newMetrics(ctx, opts)
	if err != nil {
		return nil, err
	}

	gauge, err := meter.Int64ObservableGauge("event", metric.WithDescription("Status of AWS Health events"))
	if err != nil {
		return nil, err
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		events, err := m.GetHealthEvents(ctx)
		if err != nil {
			slog.Error("failed to fetch AWS Health events", "error", err)
			return err
		}

		for _, e := range events {
			attributes := metric.WithAttributes(
				attribute.Key("region").String(aws.ToString(e.Event.Region)),
				attribute.Key("service").String(aws.ToString(e.Event.Service)),
				attribute.Key("scope").String(string(e.Event.EventScopeCode)),
				attribute.Key("category").String(string(e.Event.EventTypeCategory)),
				attribute.Key("code").String(aws.ToString(e.Event.EventTypeCode)),
			)

			status := int64(0) // closed
			if e.Event.StatusCode == healthTypes.EventStatusCodeOpen {
				status = 1 // open
			}

			if len(e.AffectedAccounts) > 0 {
				for _, account := range e.AffectedAccounts {
					o.ObserveInt64(gauge, status, attributes, metric.WithAttributes(attribute.Key("account").String(account)))
				}
			} else {
				o.ObserveInt64(gauge, status, attributes)
			}
		}

		return nil
	}, gauge)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func newMetrics(ctx context.Context, opts Options) (*Metrics, error) {
	cfg, err := newAWSConfig(ctx)
	if err != nil {
		return nil, err
	}

	m := &Metrics{
		awsconfig:           cfg,
		lastScrape:          time.Now().Add(opts.TimeShift),
		logEvents:           opts.LogEvents,
		ignoreEvents:        opts.IgnoreEvents,
		ignoreResources:     opts.IgnoreResources,
		ignoreResourceEvent: opts.IgnoreResourceEvent,
	}

	if opts.AssumeRole != "" {
		stsClient := sts.NewFromConfig(m.awsconfig)
		creds := stscreds.NewAssumeRoleProvider(stsClient, opts.AssumeRole)
		m.awsconfig.Credentials = aws.NewCredentialsCache(creds)
	}

	if err := m.newHealthClient(ctx); err != nil {
		return nil, err
	}

	if opts.SlackToken != "" && opts.SlackChannel != "" {
		m.slackAPI = slack.New(opts.SlackToken)
		m.slackChannel = opts.SlackChannel
	}

	m.organizationEnabled = m.healthOrganizationEnabled(ctx)
	if m.organizationEnabled {
		if err := m.getOrgAccountsName(ctx); err != nil {
			return nil, fmt.Errorf("listing organization accounts: %w", err)
		}
	}

	m.tz, err = time.LoadLocation(os.Getenv("TZ"))
	if err != nil {
		return nil, fmt.Errorf("loading timezone from TZ environment variable: %w", err)
	}

	// "all-regions" is kept for historical reasons, it is equivalent to not
	// filtering regions at all.
	if len(opts.Regions) > 0 && !slices.Contains(opts.Regions, "all-regions") {
		m.regions = opts.Regions
		slices.Sort(m.regions)
	}

	return m, nil
}

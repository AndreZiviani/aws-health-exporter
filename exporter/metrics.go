package exporter

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/slack-go/slack"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func NewMetrics(ctx context.Context, meter metric.Meter, c *cli.Context) (*Metrics, error) {
	m := Metrics{}

	m.init(ctx, c)

	g, _ := meter.Int64ObservableGauge("event", metric.WithDescription("Status of AWS Health events"))
	meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		events := m.GetHealthEvents()
		for _, e := range events {
			attributes := metric.WithAttributes(
				attribute.Key("region").String(aws.ToString(e.Event.Region)),
				attribute.Key("service").String(aws.ToString(e.Event.Service)),
				attribute.Key("scope").String(string(e.Event.EventScopeCode)),
				attribute.Key("category").String(string(e.Event.EventTypeCategory)),
				attribute.Key("code").String(aws.ToString(e.Event.EventTypeCode)),
			)

			status := int64(1) // open
			if e.Event.StatusCode != "open" {
				status = int64(0) // closed
			}

			if len(e.AffectedAccounts) > 0 {
				for _, account := range e.AffectedAccounts {
					o.ObserveInt64(g, status, attributes, metric.WithAttributes(attribute.Key("account").String(account)))
				}
			} else {
				o.ObserveInt64(g, status, attributes)
			}
		}

		return nil
	}, g)

	return &m, nil
}

func (m *Metrics) init(ctx context.Context, c *cli.Context) {
	cfg, err := newAWSConfig(ctx)

	if err != nil {
		panic(err.Error())
	}

	m.awsconfig = cfg

	if len(c.String("assume-role")) > 0 {
		stsclient := sts.NewFromConfig(m.awsconfig)
		creds := stscreds.NewAssumeRoleProvider(stsclient, c.String("assume-role"))
		m.awsconfig.Credentials = aws.NewCredentialsCache(creds)
	}

	m.NewHealthClient(ctx)

	m.lastScrape = time.Now().Add(c.Duration("time-shift"))

	if len(c.String("slack-token")) > 0 && len(c.String("slack-channel")) > 0 {
		m.slackToken = c.String("slack-token")
		m.slackChannel = c.String("slack-channel")
		m.slackApi = slack.New(m.slackToken)
	}

	m.organizationEnabled = m.HealthOrganizationEnabled(ctx)
	if m.organizationEnabled {
		m.GetOrgAccountsName(ctx)
	}

	m.tz, err = time.LoadLocation(os.Getenv("TZ"))
	if err != nil {
		panic(err.Error())
	}

	if c.String("regions") != "all-regions" {
		m.regions = strings.Split(c.String("regions"), ",")
		sort.Strings(m.regions)
	}

	if len(c.String("ignore-events")) > 0 {
		m.ignoreEvents = strings.Split(c.String("ignore-events"), ",")
		sort.Strings(m.ignoreEvents)
	}

	if len(c.String("ignore-resources")) > 0 {
		m.ignoreResources = strings.Split(c.String("ignore-resources"), ",")
		sort.Strings(m.ignoreResources)
	}

	if len(c.String("ignore-resource-event")) > 0 {
		m.ignoreResourceEvent = strings.Split(c.String("ignore-resource-event"), ",")
		sort.Strings(m.ignoreResourceEvent)
	}

	if c.Bool("log-events") {
		m.logEvents = true
	}
}

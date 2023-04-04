package exporter

import (
	"context"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/health"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/slack-go/slack"
	"github.com/urfave/cli/v2"
)

const (
	namespace = "aws_health"
)

func NewMetrics(ctx context.Context, registry *prometheus.Registry, c *cli.Context) (*Metrics, error) {
	m := Metrics{}

	m.init(ctx, c)

	registry.MustRegister(&m)
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	return &m, nil
}

func (m *Metrics) init(ctx context.Context, c *cli.Context) {
	cfg, err := newAWSConfig(ctx)
	if err != nil {
		panic(err.Error())
	}

	cfg.Region = "us-east-1"

	if len(c.String("assume-role")) > 0 {
		stsclient := sts.NewFromConfig(cfg)
		creds := stscreds.NewAssumeRoleProvider(stsclient, c.String("assume-role"))
		cfg.Credentials = aws.NewCredentialsCache(creds)
	}

	m.health = health.NewFromConfig(cfg)
	m.awsconfig = cfg

	m.lastScrape = time.Now()

	m.slackToken = c.String("slack-token")
	m.slackChannel = c.String("slack-channel")
	m.slackApi = slack.New(m.slackToken)

	m.organizationEnabled = m.HealthOrganizationEnabled(ctx)

	m.tz, err = time.LoadLocation(os.Getenv("TZ"))
	if err != nil {
		panic(err.Error())
	}

}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(m, ch)
}

func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	var events []HealthEvent
	if m.organizationEnabled {
		events = m.GetOrgEvents()
	} else {
		events = m.GetEvents()
	}

	if len(events) == 0 {
		return
	}

	m.SendSlackNotification(events)
}

func sanitizeLabel(label string) string {
	return strings.Replace(strings.Replace(label, ".", "_", -1), "/", "_", -1)
}

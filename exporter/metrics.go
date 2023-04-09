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

}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(m, ch)
}

func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.GetHealthEvents()
}

func sanitizeLabel(label string) string {
	return strings.Replace(strings.Replace(label, ".", "_", -1), "/", "_", -1)
}

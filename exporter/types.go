package exporter

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/slack-go/slack"
)

// Options holds every user facing configuration knob of the exporter.
type Options struct {
	Regions             []string
	AssumeRole          string
	SlackToken          string
	SlackChannel        string
	IgnoreEvents        []string
	IgnoreResources     []string
	IgnoreResourceEvent []string
	LogEvents           bool
	TimeShift           time.Duration
}

type Metrics struct {
	health *health.Client

	slackAPI     *slack.Client
	slackChannel string

	tz *time.Location

	mu         sync.Mutex
	lastScrape time.Time

	awsconfig           aws.Config
	organizationEnabled bool
	regions             []string

	ignoreEvents        []string
	ignoreResources     []string
	ignoreResourceEvent []string

	accountNames map[string]string

	logEvents bool
}

type HealthEvent struct {
	Arn               *string
	AffectedAccounts  []string
	EventScope        healthTypes.EventScopeCode
	Event             *healthTypes.Event
	EventDescription  *healthTypes.EventDescription
	AffectedResources []healthTypes.AffectedEntity
}

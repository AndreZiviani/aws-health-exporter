package exporter

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/slack-go/slack"
)

type Metrics struct {
	health              *health.Client
	organizationEnabled bool
	awsconfig           aws.Config
	lastScrape          time.Time

	slackApi     *slack.Client
	slackToken   string
	slackChannel string

	tz *time.Location
}

type HealthEvent struct {
	Arn               *string
	AffectedAccounts  []string
	EventScope        healthTypes.EventScopeCode
	Event             *healthTypes.Event
	EventDescription  *healthTypes.EventDescription
	AffectedResources []healthTypes.AffectedEntity
}

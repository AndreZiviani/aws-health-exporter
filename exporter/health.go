package exporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/slack-go/slack"
)

func (m *Metrics) HealthOrganizationEnabled(ctx context.Context) bool {
	enabled, err := m.health.DescribeHealthServiceStatusForOrganization(ctx, &health.DescribeHealthServiceStatusForOrganizationInput{})

	if err == nil && *enabled.HealthServiceAccessStatusForOrganization == "ENABLED" {
		return true
	}

	return false
}

func (m Metrics) SendSlackNotification(events []HealthEvent) {
	for _, e := range events {

		resources := m.extractResources(e.AffectedResources)
		accounts := m.extractAccounts(e.AffectedAccounts)

		service := *e.Event.Service
		region := *e.Event.Region
		status := e.Event.StatusCode

		var text, color string
		attachmentFields := []slack.AttachmentField{
			{Title: "Account(s)", Value: accounts, Short: true},
			{Title: "Resource(s)", Value: resources, Short: true},
			{Title: "Service", Value: service, Short: true},
			{Title: "Region", Value: region, Short: true},
			{Title: "Start Time", Value: e.Event.StartTime.In(m.tz).String(), Short: true},
			{Title: "Status", Value: string(status), Short: true},
			{Title: "Event ARN", Value: fmt.Sprintf("`%s`", *e.Event.Arn), Short: false},
			{Title: "Updates", Value: *e.EventDescription.LatestDescription, Short: false},
		}

		if status == healthTypes.EventStatusCodeClosed {
			text = fmt.Sprintf(":heavy_check_mark:*[RESOLVED] The AWS Health issue with the %s service in the %s region is now resolved.*", service, region)
			color = "18be52"
			attachmentFields = append(attachmentFields[:6], attachmentFields[5:]...)
			attachmentFields[5] = slack.AttachmentField{Title: "End Time", Value: e.Event.EndTime.In(m.tz).String(), Short: true}
		} else {
			text = fmt.Sprintf(":rotating_light:*[NEW] AWS Health reported an issue with the %s service in the %s region.*", service, region)
			color = "danger"
		}

		attachment := slack.Attachment{
			Color:  color,
			Fields: attachmentFields,
		}

		_, _, err := m.slackApi.PostMessage(
			m.slackChannel,
			slack.MsgOptionText(text, false),
			slack.MsgOptionAttachments(attachment),
		)
		if err != nil {
			panic(err.Error())
		}
	}
}

func (m Metrics) extractResources(resources []healthTypes.AffectedEntity) string {
	if len(resources) > 0 {
		var tmp []string
		for _, r := range resources {
			tmp = append(tmp, *r.EntityValue)
		}

		resource := fmt.Sprintf("`%s`", strings.Join(tmp, ","))
		if resource == "UNKNOWN" {
			return "All resources in region"
		}

		return resource
	}

	return "All resources in region"
}

func (m Metrics) extractAccounts(accounts []string) string {
	if len(accounts) > 0 {
		return strings.Join(accounts, ",")
	} else {
		return "All accounts in region"
	}
}

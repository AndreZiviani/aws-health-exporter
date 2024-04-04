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

func (m *Metrics) GetHealthEvents() []HealthEvent {
	var tmp, events []HealthEvent

	if m.organizationEnabled {
		tmp = m.GetOrgEvents()
	} else {
		tmp = m.GetAccountEvents()
	}

	for _, e := range tmp {
		if ignoreEvents(m.ignoreEvents, *e.Event.EventTypeCode) {
			continue
		}

		if ignoreResources(m.ignoreResources, e.AffectedResources) {
			// only ignore this event if all resources are ignored
			continue
		}

		if ignoreResourceEvent(m.ignoreResourceEvent, e) {
			continue
		}

		events = append(events, e)
		m.SendSlackNotification(e)
	}

	return events
}

func (m Metrics) SendSlackNotification(e HealthEvent) {
	if m.slackApi == nil {
		return
	}

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
		if e.Event.EndTime != nil {
			attachmentFields[5] = slack.AttachmentField{Title: "End Time", Value: e.Event.EndTime.In(m.tz).String(), Short: true}
		} else {
			attachmentFields[5] = slack.AttachmentField{Title: "End Time", Value: "-", Short: true}
		}
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
		if m.organizationEnabled {
			return strings.Join(m.getAccountsNameFromIds(accounts), ",")
		} else {
			return strings.Join(accounts, ",")
		}
	} else {
		return "All accounts in region"
	}
}

func ignoreEvents(ignoredEvents []string, event string) bool {
	for _, e := range ignoredEvents {
		if e == event {
			return true
		}
	}

	return false
}

func ignoreResources(ignoredResources []string, resources []healthTypes.AffectedEntity) bool {
	if len(ignoredResources) == 0 {
		// empty ignore list
		return false
	}

	size := len(resources)

	for _, ignored := range ignoredResources {
		for _, resource := range resources {
			if *resource.EntityValue == ignored {
				size -= 1
			}
		}
	}

	if size == 0 {
		// all resources are ignored, ignoring entire alert
		return true
	}

	// not all resources are ignored
	return false
}

func ignoreResourceEvent(ignoredResourceEvent []string, event HealthEvent) bool {
	if len(ignoredResourceEvent) == 0 {
		// empty ignore list
		return false
	}

	size := len(event.AffectedResources)
	resourceIgnored := false

	for _, ignored := range ignoredResourceEvent {
		tmp := strings.Split(ignored, ":")
		ignoredEvent, ignoredResource := tmp[0], tmp[1]

		for _, resource := range event.AffectedResources {
			if *resource.EntityValue == ignoredResource && *event.Event.EventTypeCode == ignoredEvent {
				resourceIgnored = true
				size -= 1
			}
		}
	}

	if resourceIgnored && size == 0 {
		// all resources are ignored, ignoring entire alert
		return true
	}

	// not all resources are ignored
	return false
}

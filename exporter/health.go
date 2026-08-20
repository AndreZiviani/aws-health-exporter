package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
)

func (m *Metrics) healthOrganizationEnabled(ctx context.Context) bool {
	status, err := m.health.DescribeHealthServiceStatusForOrganization(ctx, &health.DescribeHealthServiceStatusForOrganizationInput{})

	return err == nil && aws.ToString(status.HealthServiceAccessStatusForOrganization) == "ENABLED"
}

// GetHealthEvents returns every AWS Health event updated since the last
// scrape, after applying the ignore filters. New events are also forwarded to
// Slack and to the log, when enabled.
func (m *Metrics) GetHealthEvents(ctx context.Context) ([]HealthEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var all []HealthEvent
	var err error

	if m.organizationEnabled {
		all, err = m.getOrgEvents(ctx)
	} else {
		all, err = m.getAccountEvents(ctx)
	}
	if err != nil {
		return nil, err
	}

	events := make([]HealthEvent, 0, len(all))
	for _, e := range all {
		if slices.Contains(m.ignoreEvents, aws.ToString(e.Event.EventTypeCode)) {
			continue
		}

		// only ignore an event if all of its resources are ignored
		if allResourcesIgnored(m.ignoreResources, e.AffectedResources) {
			continue
		}

		if ignoreResourceEvent(m.ignoreResourceEvent, e) {
			continue
		}

		events = append(events, e)
		m.sendSlackNotification(ctx, e)
		m.logEvent(e)
	}

	return events, nil
}

func (m *Metrics) logEvent(e HealthEvent) {
	if !m.logEvents {
		return
	}

	slog.Info("aws health event",
		"resources", m.extractResources(e.AffectedResources, ", "),
		"accounts", m.extractAccounts(e.AffectedAccounts),
		"service", aws.ToString(e.Event.Service),
		"region", aws.ToString(e.Event.Region),
		"status", string(e.Event.StatusCode),
		"start_time", e.Event.StartTime.In(m.tz).String(),
		"event_arn", aws.ToString(e.Event.Arn),
		"updates", aws.ToString(e.EventDescription.LatestDescription),
	)
}

// extractResources renders the affected resources joined by separator. Each
// resource is wrapped in backticks so Slack renders it verbatim instead of
// interpreting substrings such as :aws: in an ARN as an emoji shortcode.
func (m *Metrics) extractResources(resources []healthTypes.AffectedEntity, separator string) string {
	if len(resources) == 0 {
		return "All resources in region"
	}

	names := make([]string, 0, len(resources))
	for _, r := range resources {
		names = append(names, aws.ToString(r.EntityValue))
	}

	if strings.Join(names, ",") == "UNKNOWN" {
		return "All resources in region"
	}

	for i, name := range names {
		names[i] = fmt.Sprintf("`%s`", name)
	}

	return strings.Join(names, separator)
}

func (m *Metrics) extractAccounts(accounts []string) string {
	if len(accounts) == 0 {
		return "All accounts in region"
	}

	if m.organizationEnabled {
		return strings.Join(m.getAccountsNameFromIds(accounts), ", ")
	}

	return strings.Join(accounts, ", ")
}

// allResourcesIgnored reports whether every affected resource of an event is
// on the ignore list. Events without affected resources are never ignored.
func allResourcesIgnored(ignored []string, resources []healthTypes.AffectedEntity) bool {
	if len(ignored) == 0 || len(resources) == 0 {
		return false
	}

	for _, resource := range resources {
		if !slices.Contains(ignored, aws.ToString(resource.EntityValue)) {
			return false
		}
	}

	return true
}

// ignoreResourceEvent reports whether every affected resource of an event is
// covered by an "<event type>:<resource identifier>" ignore rule.
func ignoreResourceEvent(rules []string, event HealthEvent) bool {
	if len(rules) == 0 || len(event.AffectedResources) == 0 {
		return false
	}

	eventType := aws.ToString(event.Event.EventTypeCode)

	for _, resource := range event.AffectedResources {
		ignored := false
		for _, rule := range rules {
			// the resource identifier may contain ":" (e.g. an ARN), so only
			// split on the first one
			ruleEvent, ruleResource, ok := strings.Cut(rule, ":")
			if ok && ruleEvent == eventType && ruleResource == aws.ToString(resource.EntityValue) {
				ignored = true
				break
			}
		}

		if !ignored {
			return false
		}
	}

	return true
}

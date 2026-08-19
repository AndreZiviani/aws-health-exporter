package exporter

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
)

func entities(values ...string) []healthTypes.AffectedEntity {
	out := make([]healthTypes.AffectedEntity, len(values))
	for i, v := range values {
		out[i] = healthTypes.AffectedEntity{EntityValue: aws.String(v)}
	}
	return out
}

func TestAllResourcesIgnored(t *testing.T) {
	tests := []struct {
		name      string
		ignored   []string
		resources []healthTypes.AffectedEntity
		want      bool
	}{
		{"empty ignore list", nil, entities("a"), false},
		{"no resources", []string{"a"}, nil, false},
		{"all ignored", []string{"a", "b"}, entities("a", "b"), true},
		{"partially ignored", []string{"a"}, entities("a", "b"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allResourcesIgnored(tt.ignored, tt.resources); got != tt.want {
				t.Errorf("allResourcesIgnored() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIgnoreResourceEvent(t *testing.T) {
	event := func(eventType string, resources ...string) HealthEvent {
		return HealthEvent{
			Event:             &healthTypes.Event{EventTypeCode: aws.String(eventType)},
			AffectedResources: entities(resources...),
		}
	}

	tests := []struct {
		name  string
		rules []string
		event HealthEvent
		want  bool
	}{
		{"empty rules", nil, event("EV", "a"), false},
		{"no resources", []string{"EV:a"}, event("EV"), false},
		{"all resources match", []string{"EV:a", "EV:b"}, event("EV", "a", "b"), true},
		{"partial match", []string{"EV:a"}, event("EV", "a", "b"), false},
		{"different event type", []string{"OTHER:a"}, event("EV", "a"), false},
		{
			"resource identifier containing colons",
			[]string{"EV:arn:aws:ec2:us-east-1:123456789012:vpn/vpn-1"},
			event("EV", "arn:aws:ec2:us-east-1:123456789012:vpn/vpn-1"),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ignoreResourceEvent(tt.rules, tt.event); got != tt.want {
				t.Errorf("ignoreResourceEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractResources(t *testing.T) {
	m := &Metrics{}

	if got := m.extractResources(nil); got != "All resources in region" {
		t.Errorf("extractResources(nil) = %q", got)
	}

	if got := m.extractResources(entities("UNKNOWN")); got != "All resources in region" {
		t.Errorf("extractResources(UNKNOWN) = %q", got)
	}

	if got := m.extractResources(entities("a", "b")); got != "a,b" {
		t.Errorf("extractResources(a, b) = %q", got)
	}
}

func TestSlackMessage(t *testing.T) {
	start := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	m := &Metrics{tz: time.UTC}

	newEvent := HealthEvent{
		Event: &healthTypes.Event{
			Arn:        aws.String("arn:aws:health:us-east-1::event/EC2/X/Y"),
			Service:    aws.String("EC2"),
			Region:     aws.String("us-east-1"),
			StatusCode: healthTypes.EventStatusCodeOpen,
			StartTime:  &start,
		},
		EventDescription: &healthTypes.EventDescription{LatestDescription: aws.String("something broke")},
	}

	headline, attachment := m.slackMessage(newEvent)
	if headline != "[NEW] AWS Health reported an issue with the EC2 service in the us-east-1 region." {
		t.Errorf("unexpected headline: %q", headline)
	}
	if attachment.Color != slackColorAlert {
		t.Errorf("unexpected color: %q", attachment.Color)
	}
	if len(attachment.Blocks.BlockSet) != 5 {
		t.Errorf("expected 5 blocks, got %d", len(attachment.Blocks.BlockSet))
	}

	resolvedEvent := newEvent
	resolvedEvent.Event = &healthTypes.Event{
		Arn:        newEvent.Event.Arn,
		Service:    newEvent.Event.Service,
		Region:     newEvent.Event.Region,
		StatusCode: healthTypes.EventStatusCodeClosed,
		StartTime:  &start,
		EndTime:    &end,
	}

	headline, attachment = m.slackMessage(resolvedEvent)
	if headline != "[RESOLVED] The AWS Health issue with the EC2 service in the us-east-1 region is now resolved." {
		t.Errorf("unexpected headline: %q", headline)
	}
	if attachment.Color != slackColorResolved {
		t.Errorf("unexpected color: %q", attachment.Color)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q", got)
	}

	if got := truncate("0123456789", 8); got != "01234..." {
		t.Errorf("truncate() = %q", got)
	}

	// must not split multi byte characters
	if got := truncate("aaaa日本語", 9); got != "aaaa..." {
		t.Errorf("truncate() = %q", got)
	}
}

package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/slack-go/slack"
)

const (
	slackColorResolved = "#2eb886"
	slackColorAlert    = "#d50200"

	// Slack Block Kit limits
	slackHeaderMaxLen  = 150
	slackFieldMaxLen   = 2000
	slackSectionMaxLen = 3000
)

func (m *Metrics) sendSlackNotification(ctx context.Context, e HealthEvent) {
	if m.slackAPI == nil {
		return
	}

	headline, attachment := m.slackMessage(e)

	_, _, err := m.slackAPI.PostMessageContext(
		ctx,
		m.slackChannel,
		slack.MsgOptionText(headline, false),
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		slog.Error("failed to send slack notification",
			"error", err,
			"event_arn", aws.ToString(e.Event.Arn),
		)
	}
}

func (m *Metrics) slackMessage(e HealthEvent) (string, slack.Attachment) {
	service := aws.ToString(e.Event.Service)
	region := aws.ToString(e.Event.Region)
	resolved := e.Event.StatusCode == healthTypes.EventStatusCodeClosed

	var headline, header, color string
	if resolved {
		headline = fmt.Sprintf("[RESOLVED] The AWS Health issue with the %s service in the %s region is now resolved.", service, region)
		header = fmt.Sprintf(":white_check_mark: [RESOLVED] %s in %s", service, region)
		color = slackColorResolved
	} else {
		headline = fmt.Sprintf("[NEW] AWS Health reported an issue with the %s service in the %s region.", service, region)
		header = fmt.Sprintf(":rotating_light: AWS Health: %s issue in %s", service, region)
		color = slackColorAlert
	}

	fields := []*slack.TextBlockObject{
		mrkdwnField("Account(s)", m.extractAccounts(e.AffectedAccounts)),
		mrkdwnField("Resource(s)", m.extractResources(e.AffectedResources)),
		mrkdwnField("Service", service),
		mrkdwnField("Region", region),
		mrkdwnField("Start Time", m.formatTime(e.Event.StartTime)),
	}

	if resolved {
		fields = append(fields, mrkdwnField("End Time", m.formatTime(e.Event.EndTime)))
	} else {
		fields = append(fields, mrkdwnField("Status", string(e.Event.StatusCode)))
	}

	blocks := []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, truncate(header, slackHeaderMaxLen), true, false)),
		slack.NewSectionBlock(nil, fields, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				truncate("*Updates*\n"+aws.ToString(e.EventDescription.LatestDescription), slackSectionMaxLen),
				false, false),
			nil, nil),
		slack.NewContextBlock("",
			slack.NewTextBlockObject(slack.MarkdownType,
				truncate(fmt.Sprintf("Event ARN: `%s`", aws.ToString(e.Event.Arn)), slackSectionMaxLen),
				false, false)),
	}

	return headline, slack.Attachment{
		Color:  color,
		Blocks: slack.Blocks{BlockSet: blocks},
	}
}

func (m *Metrics) formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}

	return t.In(m.tz).Format(time.RFC1123)
}

func mrkdwnField(title, value string) *slack.TextBlockObject {
	return slack.NewTextBlockObject(
		slack.MarkdownType,
		truncate(fmt.Sprintf("*%s*\n%s", title, value), slackFieldMaxLen),
		false, false,
	)
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	const ellipsis = "..."
	cut := limit - len(ellipsis)
	// avoid splitting a multi byte character
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}

	return s[:cut] + ellipsis
}

func isRuneStart(b byte) bool {
	return b&0xc0 != 0x80
}

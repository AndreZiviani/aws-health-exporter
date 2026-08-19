package exporter

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
)

func (m *Metrics) getAccountEvents(ctx context.Context) ([]HealthEvent, error) {
	now := time.Now()
	pag := health.NewDescribeEventsPaginator(
		m.health,
		&health.DescribeEventsInput{
			Filter: &healthTypes.EventFilter{
				LastUpdatedTimes: []healthTypes.DateTimeRange{
					{
						From: &m.lastScrape,
						To:   &now,
					},
				},
				Regions: m.regions,
			},
		})

	updatedEvents := make([]HealthEvent, 0)

	for pag.HasMorePages() {
		events, err := pag.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing events: %w", err)
		}

		for _, event := range events.Events {
			enriched, err := m.enrichEvent(ctx, event)
			if err != nil {
				return nil, err
			}
			updatedEvents = append(updatedEvents, enriched)
		}
	}

	m.lastScrape = now

	return updatedEvents, nil
}

func (m *Metrics) enrichEvent(ctx context.Context, event healthTypes.Event) (HealthEvent, error) {
	enriched := HealthEvent{Arn: event.Arn}

	if err := m.getEventDetails(ctx, event, &enriched); err != nil {
		return HealthEvent{}, err
	}

	if err := m.getAffectedEntities(ctx, event, &enriched); err != nil {
		return HealthEvent{}, err
	}

	return enriched, nil
}

func (m *Metrics) getEventDetails(ctx context.Context, event healthTypes.Event, enriched *HealthEvent) error {
	details, err := m.health.DescribeEventDetails(ctx, &health.DescribeEventDetailsInput{EventArns: []string{aws.ToString(event.Arn)}})
	if err != nil {
		return fmt.Errorf("describing event details of %s: %w", aws.ToString(event.Arn), err)
	}

	if len(details.SuccessfulSet) == 0 {
		return fmt.Errorf("no details available for event %s (%d failed)", aws.ToString(event.Arn), len(details.FailedSet))
	}

	enriched.Event = details.SuccessfulSet[0].Event
	enriched.EventDescription = details.SuccessfulSet[0].EventDescription

	return nil
}

func (m *Metrics) getAffectedEntities(ctx context.Context, event healthTypes.Event, enriched *HealthEvent) error {
	pag := health.NewDescribeAffectedEntitiesPaginator(
		m.health,
		&health.DescribeAffectedEntitiesInput{Filter: &healthTypes.EntityFilter{EventArns: []string{aws.ToString(event.Arn)}}})

	for pag.HasMorePages() {
		resources, err := pag.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing affected entities of %s: %w", aws.ToString(event.Arn), err)
		}

		enriched.AffectedResources = append(enriched.AffectedResources, resources.Entities...)
	}

	enriched.EventScope = event.EventScopeCode

	return nil
}

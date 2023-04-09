package exporter

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
)

func (m *Metrics) GetAccountEvents() []HealthEvent {
	ctx := context.TODO()
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
			panic(err.Error())
		}

		for _, event := range events.Events {
			enrichedEvent := m.EnrichEvents(ctx, event)
			updatedEvents = append(updatedEvents, enrichedEvent)
		}
	}

	m.lastScrape = now

	return updatedEvents
}

func (m *Metrics) EnrichEvents(ctx context.Context, event healthTypes.Event) HealthEvent {

	enrichedEvent := HealthEvent{Arn: event.Arn}

	m.getEventDetails(ctx, event, &enrichedEvent)

	m.getAffectedEntities(ctx, event, &enrichedEvent)

	return enrichedEvent
}

func (m Metrics) getEventDetails(ctx context.Context, event healthTypes.Event, enrichedEvent *HealthEvent) {
	details, err := m.health.DescribeEventDetails(ctx, &health.DescribeEventDetailsInput{EventArns: []string{*event.Arn}})
	if err != nil {
		panic(err.Error())
	}

	enrichedEvent.Event = details.SuccessfulSet[0].Event
	enrichedEvent.EventDescription = details.SuccessfulSet[0].EventDescription
}

func (m Metrics) getAffectedEntities(ctx context.Context, event healthTypes.Event, enrichedEvent *HealthEvent) {
	pagResources := health.NewDescribeAffectedEntitiesPaginator(
		m.health,
		&health.DescribeAffectedEntitiesInput{Filter: &healthTypes.EntityFilter{EventArns: []string{*event.Arn}}})

	for pagResources.HasMorePages() {
		resources, err := pagResources.NextPage(ctx)
		if err != nil {
			panic(err.Error())
		}

		enrichedEvent.AffectedResources = append(enrichedEvent.AffectedResources, resources.Entities...)
	}

	enrichedEvent.EventScope = event.EventScopeCode
}

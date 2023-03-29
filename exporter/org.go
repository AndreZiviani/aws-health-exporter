package exporter

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
)

func (m *Metrics) GetOrgEvents() []HealthEvent {
	ctx := context.TODO()
	now := time.Now()
	pag := health.NewDescribeEventsForOrganizationPaginator(
		m.health,
		&health.DescribeEventsForOrganizationInput{
			Filter: &healthTypes.OrganizationEventFilter{
				EndTime: &healthTypes.DateTimeRange{
					From: &m.lastScrape,
					To:   &now,
				},
			},
		})

	updatedEvents := make([]HealthEvent, 0)

	for pag.HasMorePages() {
		events, err := pag.NextPage(ctx)
		if err != nil {
			panic(err.Error())
		}

		for _, event := range events.Events {
			enrichedOrgEvent := m.EnrichOrgEvents(ctx, event)
			updatedEvents = append(updatedEvents, enrichedOrgEvent)
		}
	}

	m.lastScrape = now

	return updatedEvents
}

func (m *Metrics) EnrichOrgEvents(ctx context.Context, event healthTypes.OrganizationEvent) HealthEvent {

	enrichedEvent := HealthEvent{Arn: event.Arn}

	m.getAffectedAccountsForOrg(ctx, event, &enrichedEvent)

	m.getEventDetailsForOrg(ctx, event, &enrichedEvent)

	m.getAffectedEntitiesForOrg(ctx, event, &enrichedEvent)

	return enrichedEvent
}

func (m Metrics) getAffectedAccountsForOrg(ctx context.Context, event healthTypes.OrganizationEvent, enrichedEvent *HealthEvent) {
	pag := health.NewDescribeAffectedAccountsForOrganizationPaginator(
		m.health,
		&health.DescribeAffectedAccountsForOrganizationInput{EventArn: event.Arn})

	for pag.HasMorePages() {
		accounts, err := pag.NextPage(ctx)
		if err != nil {
			panic(err.Error())
		}

		enrichedEvent.EventScope = accounts.EventScopeCode
		enrichedEvent.AffectedAccounts = append(enrichedEvent.AffectedAccounts, accounts.AffectedAccounts...)
	}
}

func (m Metrics) getEventDetailsForOrg(ctx context.Context, event healthTypes.OrganizationEvent, enrichedEvent *HealthEvent) {
	var accountId *string
	if enrichedEvent.EventScope == healthTypes.EventScopeCodeAccountSpecific {
		accountId = &enrichedEvent.AffectedAccounts[0]
	}

	details, err := m.health.DescribeEventDetailsForOrganization(ctx, &health.DescribeEventDetailsForOrganizationInput{
		OrganizationEventDetailFilters: []healthTypes.EventAccountFilter{{EventArn: event.Arn, AwsAccountId: accountId}},
	})
	if err != nil {
		panic(err.Error())
	}

	enrichedEvent.Event = details.SuccessfulSet[0].Event
	enrichedEvent.EventDescription = details.SuccessfulSet[0].EventDescription
}

func (m Metrics) getAffectedEntitiesForOrg(ctx context.Context, event healthTypes.OrganizationEvent, enrichedEvent *HealthEvent) {
	var pagResources *health.DescribeAffectedEntitiesForOrganizationPaginator
	if len(enrichedEvent.AffectedAccounts) > 0 {
		accountFilter := make([]healthTypes.EventAccountFilter, len(enrichedEvent.AffectedAccounts))
		for i, account := range enrichedEvent.AffectedAccounts {
			accountFilter[i] = healthTypes.EventAccountFilter{EventArn: event.Arn, AwsAccountId: &account}
		}

		pagResources = health.NewDescribeAffectedEntitiesForOrganizationPaginator(
			m.health,
			&health.DescribeAffectedEntitiesForOrganizationInput{OrganizationEntityFilters: accountFilter})
	} else {
		pagResources = health.NewDescribeAffectedEntitiesForOrganizationPaginator(
			m.health,
			&health.DescribeAffectedEntitiesForOrganizationInput{OrganizationEntityFilters: []healthTypes.EventAccountFilter{{EventArn: event.Arn}}})
	}

	for pagResources.HasMorePages() {
		resources, err := pagResources.NextPage(ctx)
		if err != nil {
			panic(err.Error())
		}

		enrichedEvent.AffectedResources = append(enrichedEvent.AffectedResources, resources.Entities...)
	}
}

package exporter

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
)

func (m *Metrics) GetOrgEvents() []HealthEvent {
	ctx := context.TODO()
	now := time.Now()
	pag := health.NewDescribeEventsForOrganizationPaginator(
		m.health,
		&health.DescribeEventsForOrganizationInput{
			Filter: &healthTypes.OrganizationEventFilter{
				LastUpdatedTime: &healthTypes.DateTimeRange{
					From: &m.lastScrape,
					To:   &now,
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
	pagResources := make([]*health.DescribeAffectedEntitiesForOrganizationPaginator, 0)
	if len(enrichedEvent.AffectedAccounts) > 0 {
		affectedAccountsSlices := m.splitSlice(enrichedEvent.AffectedAccounts, 10)
		for _, slice := range affectedAccountsSlices {
			accountFilter := make([]healthTypes.EventAccountFilter, len(slice))
			for i, account := range slice {
				accountFilter[i] = healthTypes.EventAccountFilter{EventArn: event.Arn, AwsAccountId: &account}
			}

			pagResources = append(pagResources, health.NewDescribeAffectedEntitiesForOrganizationPaginator(
				m.health,
				&health.DescribeAffectedEntitiesForOrganizationInput{OrganizationEntityFilters: accountFilter},
			),
			)
		}
	} else {
		pagResources = append(pagResources, health.NewDescribeAffectedEntitiesForOrganizationPaginator(
			m.health,
			&health.DescribeAffectedEntitiesForOrganizationInput{OrganizationEntityFilters: []healthTypes.EventAccountFilter{{EventArn: event.Arn}}},
		),
		)
	}

	for _, slices := range pagResources {
		for slices.HasMorePages() {
			resources, err := slices.NextPage(ctx)
			if err != nil {
				panic(err.Error())
			}

			enrichedEvent.AffectedResources = append(enrichedEvent.AffectedResources, resources.Entities...)
		}
	}
}

func (m *Metrics) GetOrgAccountsName(ctx context.Context) {
	org := organizations.NewFromConfig(m.awsconfig)
	pag := organizations.NewListAccountsPaginator(
		org,
		&organizations.ListAccountsInput{},
	)

	m.accountNames = make(map[string]string, 0)

	for pag.HasMorePages() {
		accounts, err := pag.NextPage(ctx)
		if err != nil {
			panic(err.Error())
		}

		for _, account := range accounts.Accounts {
			m.accountNames[*account.Id] = *account.Name
		}
	}
}

func (m Metrics) getAccountsNameFromIds(ids []string) []string {
	names := make([]string, len(ids))
	for i, account := range ids {
		if name, ok := m.accountNames[account]; ok {
			names[i] = name
		} else {
			names[i] = account
		}
	}

	return names
}

func (m Metrics) splitSlice(slice []string, batchSize int) [][]string {
	batches := make([][]string, 0, (len(slice)+batchSize-1)/batchSize)
	for batchSize < len(slice) {
		slice, batches = slice[batchSize:], append(batches, slice[0:batchSize:batchSize])
	}
	batches = append(batches, slice)

	return batches
}

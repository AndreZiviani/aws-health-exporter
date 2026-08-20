package exporter

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/health"
	healthTypes "github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
)

func (m *Metrics) getOrgEvents(ctx context.Context) ([]HealthEvent, error) {
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
			return nil, fmt.Errorf("describing organization events: %w", err)
		}

		for _, event := range events.Events {
			enriched, err := m.enrichOrgEvent(ctx, event)
			if err != nil {
				return nil, err
			}
			updatedEvents = append(updatedEvents, enriched)
		}
	}

	m.lastScrape = now

	return updatedEvents, nil
}

func (m *Metrics) enrichOrgEvent(ctx context.Context, event healthTypes.OrganizationEvent) (HealthEvent, error) {
	enriched := HealthEvent{Arn: event.Arn}

	if err := m.getAffectedAccountsForOrg(ctx, event, &enriched); err != nil {
		return HealthEvent{}, err
	}

	if err := m.getEventDetailsForOrg(ctx, event, &enriched); err != nil {
		return HealthEvent{}, err
	}

	if err := m.getAffectedEntitiesForOrg(ctx, event, &enriched); err != nil {
		return HealthEvent{}, err
	}

	return enriched, nil
}

func (m *Metrics) getAffectedAccountsForOrg(ctx context.Context, event healthTypes.OrganizationEvent, enriched *HealthEvent) error {
	pag := health.NewDescribeAffectedAccountsForOrganizationPaginator(
		m.health,
		&health.DescribeAffectedAccountsForOrganizationInput{EventArn: event.Arn})

	for pag.HasMorePages() {
		accounts, err := pag.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing affected accounts of %s: %w", aws.ToString(event.Arn), err)
		}

		enriched.EventScope = accounts.EventScopeCode
		enriched.AffectedAccounts = append(enriched.AffectedAccounts, accounts.AffectedAccounts...)
	}

	return nil
}

func (m *Metrics) getEventDetailsForOrg(ctx context.Context, event healthTypes.OrganizationEvent, enriched *HealthEvent) error {
	var accountID *string
	if enriched.EventScope == healthTypes.EventScopeCodeAccountSpecific && len(enriched.AffectedAccounts) > 0 {
		accountID = &enriched.AffectedAccounts[0]
	}

	details, err := m.health.DescribeEventDetailsForOrganization(ctx, &health.DescribeEventDetailsForOrganizationInput{
		OrganizationEventDetailFilters: []healthTypes.EventAccountFilter{{EventArn: event.Arn, AwsAccountId: accountID}},
	})
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

func (m *Metrics) getAffectedEntitiesForOrg(ctx context.Context, event healthTypes.OrganizationEvent, enriched *HealthEvent) error {
	// DescribeAffectedEntitiesForOrganization accepts at most 10 filters per
	// request, so query the affected accounts in batches.
	var filters [][]healthTypes.EntityAccountFilter

	if len(enriched.AffectedAccounts) > 0 {
		for accounts := range slices.Chunk(enriched.AffectedAccounts, 10) {
			filter := make([]healthTypes.EntityAccountFilter, len(accounts))
			for i := range accounts {
				filter[i] = healthTypes.EntityAccountFilter{EventArn: event.Arn, AwsAccountId: &accounts[i]}
			}
			filters = append(filters, filter)
		}
	} else {
		filters = append(filters, []healthTypes.EntityAccountFilter{{EventArn: event.Arn}})
	}

	for _, filter := range filters {
		pag := health.NewDescribeAffectedEntitiesForOrganizationPaginator(
			m.health,
			&health.DescribeAffectedEntitiesForOrganizationInput{OrganizationEntityAccountFilters: filter},
		)

		for pag.HasMorePages() {
			resources, err := pag.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("describing affected entities of %s: %w", aws.ToString(event.Arn), err)
			}

			enriched.AffectedResources = append(enriched.AffectedResources, resources.Entities...)
		}
	}

	return nil
}

func (m *Metrics) getOrgAccountsName(ctx context.Context) error {
	org := organizations.NewFromConfig(m.awsconfig)
	pag := organizations.NewListAccountsPaginator(org, &organizations.ListAccountsInput{})

	m.accountNames = make(map[string]string)

	for pag.HasMorePages() {
		accounts, err := pag.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, account := range accounts.Accounts {
			m.accountNames[aws.ToString(account.Id)] = aws.ToString(account.Name)
		}
	}

	return nil
}

func (m *Metrics) getAccountsNameFromIds(ids []string) []string {
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

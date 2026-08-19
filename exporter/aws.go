package exporter

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/health"
)

const healthEndpoint = "global.health.amazonaws.com"

// newHealthClient creates the AWS Health client pointing at the currently
// active endpoint.
//
// AWS Health is a global service with two regions:
//
//	Active:  us-east-1
//	Passive: us-east-2
//
// When there is an incident in us-east-1 AWS can switch the endpoint to
// us-east-2, but resolving the active endpoint is up to the client. The
// active region is discovered by resolving the CNAME of the global endpoint,
// the same approach used by AWS Health Aware.
func (m *Metrics) newHealthClient(ctx context.Context) error {
	cname, err := net.DefaultResolver.LookupCNAME(ctx, healthEndpoint)
	if err != nil {
		return fmt.Errorf("resolving AWS Health endpoint: %w", err)
	}

	cname = strings.TrimSuffix(cname, ".")
	region := strings.Split(cname, ".")[1]

	cfg := m.awsconfig
	cfg.Region = region

	m.health = health.NewFromConfig(cfg, func(o *health.Options) {
		o.BaseEndpoint = aws.String("https://" + cname)
	})

	return nil
}

func newAWSConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	optFns = append(optFns,
		config.WithRetryer(func() aws.Retryer {
			return retry.NewAdaptiveMode()
		}),
	)

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, err
	}

	if cfg.Region == "" {
		return aws.Config{}, fmt.Errorf("no AWS region configured, please set the AWS_REGION environment variable")
	}

	return cfg, nil
}

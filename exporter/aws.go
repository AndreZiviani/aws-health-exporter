package exporter

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/health"
)

const (
	TermOnDemand   string = "JRTCKXETXF"
	TermPerHour    string = "6YS6EN2CT7"
	HealthEndpoint string = "global.health.amazonaws.com"
)

type Pricing struct {
	Product     Product
	ServiceCode string
	Terms       Terms
}

type Terms struct {
	OnDemand map[string]SKU
	Reserved map[string]SKU
}
type Product struct {
	ProductFamily string
	Attributes    map[string]string
	Sku           string
}

type SKU struct {
	PriceDimensions map[string]Details
	Sku             string
	EffectiveDate   string
	OfferTermCode   string
	TermAttributes  string
}

type Details struct {
	Unit         string
	EndRange     string
	Description  string
	AppliesTo    []string
	RateCode     string
	BeginRange   string
	PricePerUnit map[string]string
}

func (m *Metrics) NewHealthClient(ctx context.Context) {
	// AWS Health is a global service with two regions:
	// Active: us-east-1
	// Passive: us-east-2
	// When theres an incident in us-east-1 AWS can change the endpoint to us-east-2 but, AFAIK you have to manage this yourself
	// AWS Health Aware implementation also do something like this...
	cname, err := net.LookupCNAME(HealthEndpoint)
	if err != nil {
		panic(err.Error())
	}

	cname = strings.TrimSuffix(cname, ".")
	region := strings.Split(cname, ".")[1]

	cfg := m.awsconfig
	cfg.Region = region

	m.health = health.NewFromConfig(cfg, health.WithEndpointResolver(health.EndpointResolverFromURL(fmt.Sprintf("https://%s", cname))))
}

func newAWSConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		return aws.Config{}, fmt.Errorf("Please configure the AWS_REGION environment variable")
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

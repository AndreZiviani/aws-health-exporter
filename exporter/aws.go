package exporter

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const (
	TermOnDemand string = "JRTCKXETXF"
	TermPerHour  string = "6YS6EN2CT7"
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

func newAWSConfig(ctx context.Context) (aws.Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		return aws.Config{}, fmt.Errorf("Please configure the AWS_REGION environment variable")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

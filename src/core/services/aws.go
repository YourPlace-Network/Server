package services

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

func AWSGetRegion() (string, error) {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	client := imds.NewFromConfig(cfg)
	// Check if IMDS is available
	_, err = client.GetMetadata(ctx, &imds.GetMetadataInput{Path: "instance-id"})
	if err != nil {
		return "", err
	}
	// Get the region
	output, err := client.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	return output.Region, nil
}

package eks

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"sync"
)

type EKS struct {
	config aws.Config

	once sync.Once
}

func NewEKS() (*EKS, error) {
	return &EKS{}, nil
}

func (s *EKS) InitializeOnce(ctx context.Context, profileFlag *string) error {
	var err error
	s.once.Do(func() {
		var cfg aws.Config
		cfg, err = GetConfig(ctx, "", "", "", "", profileFlag, nil)
		if err != nil {

			return
		}
		s.config = cfg
	})

	return err
}

func (s *EKS) GetNodeGroup(ctx context.Context, nodeGroupName string) (*types.Nodegroup, error) {
	ec2Client := ec2.NewFromConfig(s.config)
	regions, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	if err != nil {
		return nil, err
	}

	for _, r := range regions.Regions {
		s.config.Region = *r.RegionName

		eksClient := eks.NewFromConfig(s.config)
		clusters, err := eksClient.ListClusters(ctx, &eks.ListClustersInput{})
		if err != nil {
			return nil, err
		}

		for _, cluster := range clusters.Clusters {
			ndGroup, err := eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(cluster),
				NodegroupName: aws.String(nodeGroupName),
			})
			if err == nil {
				return ndGroup.Nodegroup, nil
			}
		}
	}

	return nil, fmt.Errorf("EKS Cluster for node group %s not found, please connect your aws cli to the "+
		"account which hosts the EKS cluster. specify the aws profile with --aws-cli-profile if needed.", nodeGroupName)
}

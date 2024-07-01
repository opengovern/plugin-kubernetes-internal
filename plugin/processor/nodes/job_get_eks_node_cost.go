package nodes

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
)

type GetEksNodeCost struct {
	processor *Processor
	itemId    string
}

func NewGetEksNodeCost(processor *Processor, itemId string) *GetEksNodeCost {
	return &GetEksNodeCost{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetEksNodeCost) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_eks_node_cost_for_%s", j.itemId),
		Description: fmt.Sprintf("Getting EKS node cost for %s", j.itemId),
		MaxRetry:    0,
	}
}

func (j *GetEksNodeCost) Run(ctx context.Context) error {
	return nil
}

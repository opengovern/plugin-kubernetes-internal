package nodes

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/version"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/api/core/v1"
)

type GetNodeCostJob struct {
	processor *Processor
	itemId    string
}

func NewGetNodeCostJob(processor *Processor, itemId string) *GetNodeCostJob {
	return &GetNodeCostJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetNodeCostJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_node_cost_for_%s", j.itemId),
		Description: fmt.Sprintf("Getting node cost for %s", j.itemId),
		MaxRetry:    3,
	}
}

func (j *GetNodeCostJob) Run(ctx context.Context) error {
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("node item not found")
	}

	var taints []*v1.Taint
	for _, t := range item.Node.Spec.Taints {
		taints = append(taints, &t)
	}
	node := golang.KubernetesNode{
		Id:          item.GetID(),
		Name:        item.Node.Name,
		Annotations: item.Node.Annotations,
		Labels:      item.Node.Labels,
		Capacity:    make(map[string]int64),
		NodeSystemInfo: &golang.NodeSystemInfo{
			MachineID:               item.Node.Status.NodeInfo.MachineID,
			SystemUUID:              item.Node.Status.NodeInfo.SystemUUID,
			BootID:                  item.Node.Status.NodeInfo.BootID,
			KernelVersion:           item.Node.Status.NodeInfo.KernelVersion,
			OsImage:                 item.Node.Status.NodeInfo.OSImage,
			ContainerRuntimeVersion: item.Node.Status.NodeInfo.ContainerRuntimeVersion,
			KubeletVersion:          item.Node.Status.NodeInfo.KubeletVersion,
			KubeProxyVersion:        item.Node.Status.NodeInfo.KubeProxyVersion,
			OperatingSystem:         item.Node.Status.NodeInfo.OperatingSystem,
			Architecture:            item.Node.Status.NodeInfo.Architecture,
		},
		Taints: taints,
	}
	for k, v := range item.Node.Status.Capacity {
		node.Capacity[fmt.Sprintf("%v", k)] = v.Value()
	}

	var request = &golang.KubernetesNodeGetCostRequest{
		RequestId:      wrapperspb.String(uuid.New().String()),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Node:           &node,
	}
	grpcCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, shared.GrpcOptimizeRequestTimeout)
	defer cancel()
	response, err := j.processor.client.KubernetesNodeGetCost(grpcCtx, request)
	if err != nil {
		if grpcErr, ok := status.FromError(err); ok && grpcErr.Code() == codes.InvalidArgument {
			return nil
		} else {
			return err
		}
	}

	item.CostResponse = response
	j.processor.items.Set(j.itemId, item)

	return nil
}

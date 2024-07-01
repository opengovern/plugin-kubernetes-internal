package nodes

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
)

type ClusterType int

const (
	ClusterTypeUnknown ClusterType = iota
	ClusterTypeAwsEks
	ClusterTypeAzureAks
	ClusterTypeGoogleGke
)

type NodeItem struct {
	Node        corev1.Node
	ClusterType ClusterType

	Skipped            bool
	SkipReason         string
	LazyLoadingEnabled bool
}

func (i NodeItem) GetID() string {
	return fmt.Sprintf("corev1.node/%s", i.Node.Name)
}

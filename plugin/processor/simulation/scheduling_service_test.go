package simulation

import (
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/api/resource"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	v1 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/core/v1"
)

func TestServiceAddDaemonSet(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 0,
			AllocatedMem: 0,
			AllocatedPod: 0,
			Labels:       map[string]string{"zone": "us-west1"},
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 0,
			AllocatedMem: 0,
			AllocatedPod: 0,
			Labels:       map[string]string{"zone": "us-west2"},
		},
	}

	scheduler := NewSchedulerService(nodes)

	memory := 0.2 * float64(GB)
	daemonSet := v1.DaemonSet{
		ObjectMeta: v12.ObjectMeta{
			Name:      "daemonset-1",
			Namespace: "ns-1",
		},
		Spec: v1.DaemonSetSpec{
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					Containers: []v13.Container{
						{
							Resources: v13.ResourceRequirements{
								Requests: v13.ResourceList{
									v13.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
									v13.ResourceMemory: *resource.NewQuantity(int64(memory), resource.BinarySI),
								},
							},
						},
					},
				},
			},
		},
	}

	scheduler.AddDaemonSet(daemonSet)

	deployment := v1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "deployment-1",
			Namespace: "ns-1",
		},
		Spec: v1.DeploymentSpec{
			Replicas: proto.Int32(2),
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					Containers: []v13.Container{
						{
							Resources: v13.ResourceRequirements{
								Requests: v13.ResourceList{
									v13.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
									v13.ResourceMemory: *resource.NewQuantity(int64(memory), resource.BinarySI),
								},
							},
						},
					},
				},
			},
		},
	}
	scheduler.AddDeployment(deployment)
	removed, err := scheduler.Simulate()
	assert.NoError(t, err)
	assert.Len(t, removed, 1)

	scheduler.AddDeployment(deployment)
	removed, err = scheduler.Simulate()
	assert.NoError(t, err)
	assert.Len(t, removed, 1)

	deployment.ObjectMeta.Name = "deployment-2"
	deployment.Spec.Replicas = proto.Int32(2)
	scheduler.AddDeployment(deployment)
	removed, err = scheduler.Simulate()
	assert.NoError(t, err)
	assert.Len(t, removed, 0)
}

package simulation

import (
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	v12 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"

	v1 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSchedulePodWithStrategy(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 2,
			AllocatedMem: 4,
			AllocatedPod: 50,
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 1,
			AllocatedMem: 2,
			AllocatedPod: 25,
		},
	}

	scheduler := New(nodes)

	pod := v13.PodTemplateSpec{
		Spec: v13.PodSpec{
			Containers: []v13.Container{
				{
					Resources: v13.ResourceRequirements{
						Requests: v13.ResourceList{
							v13.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
							v13.ResourceMemory: *resource.NewQuantity(1*GB, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	success, _ := scheduler.schedulePodWithStrategy(pod)

	if !success {
		t.Errorf("Failed to schedule pod")
	}

	if scheduler.nodes[0].AllocatedCPU != 2.5 {
		t.Errorf("Expected AllocatedCPU to be 2.5, got %f", scheduler.nodes[0].AllocatedCPU)
	}

	if scheduler.nodes[0].AllocatedMem != 5 {
		t.Errorf("Expected AllocatedMem to be 5, got %f", scheduler.nodes[0].AllocatedMem)
	}

	if scheduler.nodes[0].AllocatedPod != 51 {
		t.Errorf("Expected AllocatedPod to be 51, got %d", scheduler.nodes[0].AllocatedPod)
	}
}

func TestCanRemoveNode(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 2,
			AllocatedMem: 4,
			AllocatedPod: 50,
			Pods: []v13.PodTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test"},
					},
					Spec: v13.PodSpec{
						Containers: []v13.Container{
							{
								Resources: v13.ResourceRequirements{
									Requests: v13.ResourceList{
										v13.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
										v13.ResourceMemory: *resource.NewQuantity(1*GB, resource.BinarySI),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 1,
			AllocatedMem: 2,
			AllocatedPod: 25,
		},
	}

	scheduler := New(nodes)

	canRemove, err := scheduler.CanRemoveNode("node1")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !canRemove {
		t.Errorf("Expected to be able to remove node1")
	}

	// Test with PDB
	pdb := policyv1.PodDisruptionBudget{
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}
	scheduler.AddPodDisruptionBudget(pdb)

	canRemove, err = scheduler.CanRemoveNode("node1")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if canRemove {
		t.Errorf("Expected not to be able to remove node1 due to PDB")
	}
}

func TestTolerates(t *testing.T) {
	scheduler := New([]shared.KubernetesNode{})

	nodeTaints := []v13.Taint{
		{
			Key:    "key1",
			Value:  "value1",
			Effect: v13.TaintEffectNoSchedule,
		},
	}

	podSpec := v13.PodSpec{
		Tolerations: []v13.Toleration{
			{
				Key:    "key1",
				Value:  "value1",
				Effect: v13.TaintEffectNoSchedule,
			},
		},
	}

	if !scheduler.tolerates(podSpec, nodeTaints) {
		t.Errorf("Expected pod to tolerate node taints")
	}

	podSpec.Tolerations = []v13.Toleration{}

	if scheduler.tolerates(podSpec, nodeTaints) {
		t.Errorf("Expected pod not to tolerate node taints")
	}
}

func TestSatisfiesNodeAffinity(t *testing.T) {
	scheduler := New([]shared.KubernetesNode{})

	nodeLabels := map[string]string{
		"zone": "us-west1",
		"disk": "ssd",
	}

	nodeAffinity := &v13.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &v13.NodeSelector{
			NodeSelectorTerms: []v13.NodeSelectorTerm{
				{
					MatchExpressions: []v13.NodeSelectorRequirement{
						{
							Key:      "zone",
							Operator: v13.NodeSelectorOpIn,
							Values:   []string{"us-west1", "us-west2"},
						},
						{
							Key:      "disk",
							Operator: v13.NodeSelectorOpIn,
							Values:   []string{"ssd"},
						},
					},
				},
			},
		},
	}

	if !scheduler.satisfiesNodeAffinity(nodeAffinity, nodeLabels) {
		t.Errorf("Expected node to satisfy node affinity")
	}

	nodeLabels["disk"] = "hdd"

	if scheduler.satisfiesNodeAffinity(nodeAffinity, nodeLabels) {
		t.Errorf("Expected node not to satisfy node affinity")
	}
}

func TestIntegration(t *testing.T) {
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

	scheduler := New(nodes)

	deployment := v1.Deployment{
		Spec: v1.DeploymentSpec{
			Replicas: &[]int32{5}[0],
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					Containers: []v13.Container{
						{
							Resources: v13.ResourceRequirements{
								Requests: v13.ResourceList{
									v13.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
									v13.ResourceMemory: *resource.NewQuantity(1*GB, resource.BinarySI),
								},
							},
						},
					},
					Affinity: &v13.Affinity{
						NodeAffinity: &v13.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v13.NodeSelector{
								NodeSelectorTerms: []v13.NodeSelectorTerm{
									{
										MatchExpressions: []v13.NodeSelectorRequirement{
											{
												Key:      "zone",
												Operator: v13.NodeSelectorOpIn,
												Values:   []string{"us-west1"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	scheduler.AddDeployment(deployment)

	utilization := scheduler.GetNodeUtilization()

	if utilization["node1"]["CPU"] != 0.625 {
		t.Errorf("Expected node1 CPU utilization to be 0.625, got %f", utilization["node1"]["CPU"])
	}

	if utilization["node2"]["CPU"] != 0 {
		t.Errorf("Expected node2 CPU utilization to be 0, got %f", utilization["node2"]["CPU"])
	}

	canRemove, err := scheduler.CanRemoveNode("node2")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !canRemove {
		t.Errorf("Expected to be able to remove node2")
	}

	canRemove, err = scheduler.CanRemoveNode("node1")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if canRemove {
		t.Errorf("Expected not to be able to remove node1")
	}
}

func TestSchedulePodWithStrategyEdgeCases(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 3,
			AllocatedMem: 7,
			AllocatedPod: 109,
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 3,
			AllocatedMem: 5.9,
			AllocatedPod: 100,
		},
	}

	scheduler := New(nodes)

	// Test case 1: Pod that barely fits
	memory := 0.9 * float64(GB)
	pod1 := v13.PodTemplateSpec{
		Spec: v13.PodSpec{
			Containers: []v13.Container{
				{
					Resources: v13.ResourceRequirements{
						Requests: v13.ResourceList{
							v13.ResourceCPU:    *resource.NewMilliQuantity(400, resource.DecimalSI),
							v13.ResourceMemory: *resource.NewQuantity(int64(memory), resource.BinarySI),
						},
					},
				},
			},
		},
	}

	success, _ := scheduler.schedulePodWithStrategy(pod1)
	if !success {
		t.Errorf("Failed to schedule pod1 that barely fits")
	}

	// Test case 2: Pod that doesn't fit anywhere
	pod2 := v13.PodTemplateSpec{
		Spec: v13.PodSpec{
			Containers: []v13.Container{
				{
					Resources: v13.ResourceRequirements{
						Requests: v13.ResourceList{
							v13.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
							v13.ResourceMemory: *resource.NewQuantity(2*GB, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	success, _ = scheduler.schedulePodWithStrategy(pod2)
	if success {
		t.Errorf("Unexpectedly scheduled pod2 that shouldn't fit anywhere")
	}

	// Test case 3: Pod with no resource requests
	pod3 := v13.PodTemplateSpec{
		Spec: v13.PodSpec{
			Containers: []v13.Container{
				{
					Resources: v13.ResourceRequirements{},
				},
			},
		},
	}

	success, _ = scheduler.schedulePodWithStrategy(pod3)
	if !success {
		t.Errorf("Failed to schedule pod3 with no resource requests")
	}
}

func TestAddDaemonSet(t *testing.T) {
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

	scheduler := New(nodes)

	memory := 0.2 * float64(GB)
	daemonSet := v1.DaemonSet{
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

	for _, node := range scheduler.nodes {
		if node.AllocatedPod != 1 {
			t.Errorf("Expected 1 DaemonSet pod on node %s, got %d", node.Name, node.AllocatedPod)
		}
		if node.AllocatedCPU-0.1 > 0.001 {
			t.Errorf("Expected 0.1 CPU allocated on node %s, got %f", node.Name, node.AllocatedCPU)
		}
		if node.AllocatedMem-0.2 > 0.001 {
			t.Errorf("Expected 0.2 GB memory allocated on node %s, got %f", node.Name, node.AllocatedMem)
		}
	}
}

func TestAddJob(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 0,
			AllocatedMem: 0,
			AllocatedPod: 0,
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 0,
			AllocatedMem: 0,
			AllocatedPod: 0,
		},
	}

	scheduler := New(nodes)

	job := v12.Job{
		Spec: v12.JobSpec{
			Completions: &[]int32{3}[0],
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					Containers: []v13.Container{
						{
							Resources: v13.ResourceRequirements{
								Requests: v13.ResourceList{
									v13.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
									v13.ResourceMemory: *resource.NewQuantity(1*GB, resource.BinarySI),
								},
							},
						},
					},
				},
			},
		},
	}

	scheduler.AddJob(job)

	totalAllocatedPods := 0
	for _, node := range scheduler.nodes {
		totalAllocatedPods += node.AllocatedPod
	}

	if totalAllocatedPods != 3 {
		t.Errorf("Expected 3 Job pods scheduled, got %d", totalAllocatedPods)
	}
}

func TestGetNodeUtilization(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 2,
			AllocatedMem: 4,
			AllocatedPod: 55,
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 3,
			AllocatedMem: 6,
			AllocatedPod: 82,
		},
	}

	scheduler := New(nodes)

	utilization := scheduler.GetNodeUtilization()

	expectedUtilization := map[string]map[string]float64{
		"node1": {
			"CPU":    2 / 4.0,
			"Memory": 4 / 8.0,
			"Pods":   55 / 110.0,
		},
		"node2": {
			"CPU":    3 / 4.0,
			"Memory": 6 / 8.0,
			"Pods":   82 / 110.0,
		},
	}

	for nodeName, metrics := range utilization {
		for metricName, value := range metrics {
			expected := expectedUtilization[nodeName][metricName]
			if value != expected {
				t.Errorf("Expected %s utilization for %s to be %f, got %f", metricName, nodeName, expected, value)
			}
		}
	}
}

func TestCanRemoveNodeWithPDBs(t *testing.T) {
	nodes := []shared.KubernetesNode{
		{
			Name:         "node1",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 2,
			AllocatedMem: 4,
			AllocatedPod: 2,
			Pods: []v13.PodTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test1-node1",
						Labels: map[string]string{"app": "test1"},
					},
					Spec: v13.PodSpec{
						Containers: []v13.Container{
							{
								Resources: v13.ResourceRequirements{
									Requests: v13.ResourceList{
										v13.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
										v13.ResourceMemory: *resource.NewQuantity(2*GB, resource.BinarySI),
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test2-node1",
						Labels: map[string]string{"app": "test2"},
					},
					Spec: v13.PodSpec{
						Containers: []v13.Container{
							{
								Resources: v13.ResourceRequirements{
									Requests: v13.ResourceList{
										v13.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
										v13.ResourceMemory: *resource.NewQuantity(2*GB, resource.BinarySI),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:         "node2",
			VCores:       4,
			Memory:       8,
			MaxPodCount:  110,
			AllocatedCPU: 1,
			AllocatedMem: 2,
			AllocatedPod: 1,
			Pods: []v13.PodTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test1-node2",
						Labels: map[string]string{"app": "test1"},
					},
					Spec: v13.PodSpec{
						Containers: []v13.Container{
							{
								Resources: v13.ResourceRequirements{
									Requests: v13.ResourceList{
										v13.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
										v13.ResourceMemory: *resource.NewQuantity(2*GB, resource.BinarySI),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	scheduler := New(nodes)

	// Add PDB that requires at least 2 pods of app "test1" to be available
	pdb1 := policyv1.PodDisruptionBudget{
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test1"},
			},
		},
	}
	scheduler.AddPodDisruptionBudget(pdb1)

	// Add PDB that requires at least 1 pod of app "test2" to be available
	pdb2 := policyv1.PodDisruptionBudget{
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test2"},
			},
		},
	}
	scheduler.AddPodDisruptionBudget(pdb2)

	// Test removing node1 (should fail due to PDBs)
	canRemove, err := scheduler.CanRemoveNode("node1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if canRemove {
		t.Errorf("Expected not to be able to remove node1 due to PDBs")
	}

	// Test removing node2 (should succeed)
	canRemove, err = scheduler.CanRemoveNode("node2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !canRemove {
		t.Errorf("Expected to be able to remove node2")
	}
	// Add a new pod to node2 to make it non-removable
	scheduler.nodes[1].Pods = append(scheduler.nodes[1].Pods, v13.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "test2"},
		},
		Spec: v13.PodSpec{
			Containers: []v13.Container{
				{
					Resources: v13.ResourceRequirements{
						Requests: v13.ResourceList{
							v13.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
							v13.ResourceMemory: *resource.NewQuantity(2*GB, resource.BinarySI),
						},
					},
				},
			},
		},
	})
	scheduler.nodes[1].AllocatedCPU += 1
	scheduler.nodes[1].AllocatedMem += 2
	scheduler.nodes[1].AllocatedPod += 1

	// Test removing node2 again (should fail now)
	canRemove, err = scheduler.CanRemoveNode("node2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if canRemove {
		t.Errorf("Expected not to be able to remove node2")
	}
}

func TestPodScheduling(t *testing.T) {
	t.Run("Pod with no resource requests", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(1, 1024, 10)
		pod := createPod("no-resource-pod", 0, 0)

		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod with no resource requests")
		}
		if node.AllocatedPod != 1 {
			t.Errorf("Expected 1 allocated pod, got %d", node.AllocatedPod)
		}
	})

	t.Run("Pod that barely fits on a node", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(1, 1024, 10)
		pod := createPod("barely-fits-pod", 0.84, 870)

		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod that barely fits")
		}
		if node.AllocatedPod != 1 {
			t.Errorf("Expected 1 allocated pod, got %d", node.AllocatedPod)
		}
	})

	t.Run("Pod that exceeds node capacity", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(1, 1024, 10)
		pod := createPod("exceeds-capacity-pod", 2, 2048)

		success, _ := scheduler.AddPod(pod)

		if success {
			t.Errorf("Incorrectly scheduled pod that exceeds node capacity")
		}
		if node.AllocatedPod != 0 {
			t.Errorf("Expected 0 allocated pods, got %d", node.AllocatedPod)
		}
	})

	t.Run("Pod with very large resource requests", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(8, 16384, 100)
		pod := createPod("large-resource-pod", 6, 12288)

		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod with large resource requests")
		}
		if node.AllocatedPod != 1 {
			t.Errorf("Expected 1 allocated pod, got %d", node.AllocatedPod)
		}
	})

	t.Run("Multiple pods filling up a node exactly", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(1, 1024, 10)
		pod1 := createPod("fill-pod-1", 0.4, 400)
		pod2 := createPod("fill-pod-2", 0.3, 300)
		pod3 := createPod("fill-pod-3", 0.15, 170)

		success1, _ := scheduler.AddPod(pod1)
		success2, _ := scheduler.AddPod(pod2)
		success3, _ := scheduler.AddPod(pod3)

		if !success1 || !success2 || !success3 {
			t.Errorf("Failed to schedule pods filling up the node")
		}
		if node.AllocatedPod != 3 {
			t.Errorf("Expected 3 allocated pods, got %d", node.AllocatedPod)
		}

		// Try to add one more pod that should not fit
		podExcess := createPod("excess-pod", 0.01, 1)
		successExcess, _ := scheduler.AddPod(podExcess)

		if successExcess {
			t.Errorf("Incorrectly scheduled pod exceeding node capacity")
		}
		if node.AllocatedPod != 3 {
			t.Errorf("Expected 3 allocated pods after failed scheduling, got %d", node.AllocatedPod)
		}
	})
}

func TestNodeConditions(t *testing.T) {
	t.Run("Completely empty node", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(4, 8192, 10)

		if node.AllocatedPod != 0 {
			t.Errorf("Expected 0 allocated pods, got %d", node.AllocatedPod)
		}
		if node.AllocatedCPU != 0 {
			t.Errorf("Expected 0 allocated CPU, got %f", node.AllocatedCPU)
		}
		if node.AllocatedMem != 0 {
			t.Errorf("Expected 0 allocated memory, got %f", node.AllocatedMem)
		}

		pod := createPod("test-pod", 1, 1024)
		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod on empty node")
		}
		if node.AllocatedPod != 1 {
			t.Errorf("Expected 1 allocated pod after scheduling, got %d", node.AllocatedPod)
		}
	})

	t.Run("Nearly full node", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(4, 8192, 10)

		// Fill the node to near capacity
		for i := 0; i < 8; i++ {
			pod := createPod(fmt.Sprintf("fill-pod-%d", i), 0.4, 800)
			scheduler.AddPod(pod)
		}

		if node.AllocatedPod != 8 {
			t.Errorf("Expected 8 allocated pods, got %d", node.AllocatedPod)
		}

		// Try to add a pod that fits
		smallPod := createPod("small-pod", 0.2, 400)
		successSmall, _ := scheduler.AddPod(smallPod)

		if !successSmall {
			t.Errorf("Failed to schedule small pod on nearly full node")
		}

		// Try to add a pod that doesn't fit
		largePod := createPod("large-pod", 1, 2000)
		successLarge, _ := scheduler.AddPod(largePod)

		if successLarge {
			t.Errorf("Incorrectly scheduled large pod on nearly full node")
		}
	})

	t.Run("Node with exactly one spot left for a pod", func(t *testing.T) {
		scheduler, node := setupSchedulerWithOneNode(4, 8192, 10)

		// Fill the node leaving space for exactly one more pod
		for i := 0; i < 8; i++ {
			pod := createPod(fmt.Sprintf("fill-pod-%d", i), 0.34, 690)
			scheduler.AddPod(pod)
		}

		if node.AllocatedPod != 8 {
			t.Errorf("Expected 8 allocated pods, got %d", node.AllocatedPod)
		}

		// Try to add a pod that fits
		fitPod := createPod("fit-pod", 0.34, 690)
		successFit, _ := scheduler.AddPod(fitPod)

		if !successFit {
			t.Errorf("Failed to schedule pod that should fit in last spot")
		}

		if node.AllocatedPod != 9 {
			t.Errorf("Expected 9 allocated pods after filling last spot, got %d", node.AllocatedPod)
		}

		// Try to add one more pod that should not fit
		extraPod := createPod("extra-pod", 0.1, 100)
		successExtra, _ := scheduler.AddPod(extraPod)

		if successExtra {
			t.Errorf("Incorrectly scheduled pod when node was already full")
		}
	})
}

func TestAffinityAndAntiAffinity(t *testing.T) {
	t.Run("Pod with node affinity that matches one node", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
			{Name: "node2", Labels: map[string]string{"zone": "us-east-1b"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		pod := createPodWithNodeAffinity("test-pod", "zone", []string{"us-east-1a"})
		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod with matching node affinity")
		}
		if scheduler.nodes[0].AllocatedPod != 1 || scheduler.nodes[1].AllocatedPod != 0 {
			t.Errorf("Pod scheduled on wrong node")
		}
	})

	t.Run("Pod with node affinity that doesn't match any nodes", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
			{Name: "node2", Labels: map[string]string{"zone": "us-east-1b"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		pod := createPodWithNodeAffinity("test-pod", "zone", []string{"us-west-1a"})
		success, _ := scheduler.AddPod(pod)

		if success {
			t.Errorf("Incorrectly scheduled pod with non-matching node affinity")
		}
	})

	t.Run("Pod with pod affinity that can be satisfied", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
			{Name: "node2", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		// Schedule a pod to satisfy affinity
		existingPod := createPodWithLabels("existing-pod", map[string]string{"app": "web"})
		scheduler.AddPod(existingPod)

		pod := createPodWithPodAffinity("test-pod", "app", "web", "zone")
		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod with satisfiable pod affinity")
		}
	})

	t.Run("Pod with pod anti-affinity that prevents scheduling", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		// Schedule a pod to trigger anti-affinity
		existingPod := createPodWithLabels("existing-pod", map[string]string{"app": "web"})
		scheduler.AddPod(existingPod)

		pod := createPodWithPodAntiAffinity("test-pod", "app", "web", "zone")
		success, _ := scheduler.AddPod(pod)

		if success {
			t.Errorf("Incorrectly scheduled pod with unsatisfiable pod anti-affinity")
		}
	})

	t.Run("Complex affinity rules involving multiple pods", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"zone": "us-east-1a", "disk": "ssd"}, MaxPodCount: 10},
			{Name: "node2", Labels: map[string]string{"zone": "us-east-1b", "disk": "hdd"}, MaxPodCount: 10},
			{Name: "node3", Labels: map[string]string{"zone": "us-east-1a", "disk": "hdd"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		// Schedule some initial pods
		scheduler.AddPod(createPodWithLabels("web-1", map[string]string{"app": "web"}))
		scheduler.AddPod(createPodWithLabels("db-1", map[string]string{"app": "db"}))

		// Create a pod with complex affinity rules
		complexPod := v13.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "complex-pod"},
			Spec: v13.PodSpec{
				Affinity: &v13.Affinity{
					NodeAffinity: &v13.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &v13.NodeSelector{
							NodeSelectorTerms: []v13.NodeSelectorTerm{
								{
									MatchExpressions: []v13.NodeSelectorRequirement{
										{
											Key:      "disk",
											Operator: v13.NodeSelectorOpIn,
											Values:   []string{"ssd"},
										},
									},
								},
							},
						},
					},
					PodAffinity: &v13.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v13.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "web"},
								},
								TopologyKey: "zone",
							},
						},
					},
					PodAntiAffinity: &v13.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v13.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "db"},
								},
								TopologyKey: "zone",
							},
						},
					},
				},
			},
		}

		success, _ := scheduler.AddPod(complexPod)

		if success {
			t.Errorf("Schedule should fail with complex affinity rules")
		}
	})
}

func TestTaintsAndTolerations(t *testing.T) {
	t.Run("Pod with no tolerations on a tainted node", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{
				Name: "tainted-node",
				Taints: []v13.Taint{
					{
						Key:    "key1",
						Value:  "value1",
						Effect: v13.TaintEffectNoSchedule,
					},
				},
			},
		}
		scheduler := New(nodes)

		pod := createPod("test-pod", 1, 1024)
		success, _ := scheduler.AddPod(pod)

		if success {
			t.Errorf("Incorrectly scheduled pod without tolerations on tainted node")
		}
	})

	t.Run("Pod with matching toleration on a tainted node", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{
				Name: "tainted-node",
				Taints: []v13.Taint{
					{
						Key:    "key1",
						Value:  "value1",
						Effect: v13.TaintEffectNoSchedule,
					},
				},
				MaxPodCount: 10,
			},
		}
		scheduler := New(nodes)

		pod := createPodWithToleration("test-pod", "key1", "value1", v13.TaintEffectNoSchedule)
		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod with matching toleration on tainted node")
		}
		if scheduler.nodes[0].AllocatedPod != 1 {
			t.Errorf("Expected 1 allocated pod, got %d", scheduler.nodes[0].AllocatedPod)
		}
	})

	t.Run("Pod with non-matching toleration on a tainted node", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{
				Name: "tainted-node",
				Taints: []v13.Taint{
					{
						Key:    "key1",
						Value:  "value1",
						Effect: v13.TaintEffectNoSchedule,
					},
				},
				MaxPodCount: 10,
			},
		}
		scheduler := New(nodes)

		pod := createPodWithToleration("test-pod", "key2", "value2", v13.TaintEffectNoSchedule)
		success, _ := scheduler.AddPod(pod)

		if success {
			t.Errorf("Incorrectly scheduled pod with non-matching toleration on tainted node")
		}
	})

	t.Run("Pod with matching key but non-matching value toleration on a tainted node", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{
				Name: "tainted-node",
				Taints: []v13.Taint{
					{
						Key:    "key1",
						Value:  "value1",
						Effect: v13.TaintEffectNoSchedule,
					},
				},
			},
		}
		scheduler := New(nodes)

		pod := createPodWithToleration("test-pod", "key1", "value2", v13.TaintEffectNoSchedule)
		success, _ := scheduler.AddPod(pod)

		if success {
			t.Errorf("Incorrectly scheduled pod with matching key but non-matching value toleration on tainted node")
		}
	})

	t.Run("Pod with matching key and effect but no value toleration on a tainted node", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{
				Name: "tainted-node",
				Taints: []v13.Taint{
					{
						Key:    "key1",
						Value:  "value1",
						Effect: v13.TaintEffectNoSchedule,
					},
				},
				MaxPodCount: 10,
			},
		}
		scheduler := New(nodes)

		pod := createPodWithTolerationNoValue("test-pod", "key1", v13.TaintEffectNoSchedule)
		success, _ := scheduler.AddPod(pod)

		if !success {
			t.Errorf("Failed to schedule pod with matching key and effect but no value toleration on tainted node")
		}
		if scheduler.nodes[0].AllocatedPod != 1 {
			t.Errorf("Expected 1 allocated pod, got %d", scheduler.nodes[0].AllocatedPod)
		}
	})
}

func TestSpecialPodTypes(t *testing.T) {
	t.Run("DaemonSet pod scheduling on all nodes", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", MaxPodCount: 10},
			{Name: "node2", MaxPodCount: 10},
			{Name: "node3", MaxPodCount: 10},
		}
		scheduler := New(nodes)

		daemonSet := createDaemonSet("test-daemonset", nil)
		scheduler.AddDaemonSet(daemonSet)

		for i, node := range scheduler.nodes {
			if node.AllocatedPod != 1 {
				t.Errorf("Node %d should have 1 DaemonSet pod, but has %d", i, node.AllocatedPod)
			}
		}
	})

	t.Run("DaemonSet pod scheduling on nodes with matching node selector", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"role": "worker"}, MaxPodCount: 10},
			{Name: "node2", Labels: map[string]string{"role": "master"}, MaxPodCount: 10},
			{Name: "node3", Labels: map[string]string{"role": "worker"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		nodeSelector := map[string]string{"role": "worker"}
		daemonSet := createDaemonSet("test-daemonset", nodeSelector)
		if ok, _ := scheduler.AddDaemonSet(daemonSet); !ok {
			t.Errorf("Failed to add daemonset")
		}

		expectedPods := []int{1, 0, 1}
		for i, node := range scheduler.nodes {
			if node.AllocatedPod != expectedPods[i] {
				t.Errorf("Node %d should have %d DaemonSet pod(s), but has %d", i, expectedPods[i], node.AllocatedPod)
			}
		}
	})

	t.Run("DaemonSet pod scheduling with node affinity", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
			{Name: "node2", Labels: map[string]string{"zone": "us-east-1b"}, MaxPodCount: 10},
			{Name: "node3", Labels: map[string]string{"zone": "us-east-1a"}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		affinity := &v13.Affinity{
			NodeAffinity: &v13.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v13.NodeSelector{
					NodeSelectorTerms: []v13.NodeSelectorTerm{
						{
							MatchExpressions: []v13.NodeSelectorRequirement{
								{
									Key:      "zone",
									Operator: v13.NodeSelectorOpIn,
									Values:   []string{"us-east-1a"},
								},
							},
						},
					},
				},
			},
		}
		daemonSet := createDaemonSetWithAffinity("test-daemonset", affinity)
		scheduler.AddDaemonSet(daemonSet)

		expectedPods := []int{1, 0, 1}
		for i, node := range scheduler.nodes {
			if node.AllocatedPod != expectedPods[i] {
				t.Errorf("Node %d should have %d DaemonSet pod(s), but has %d", i, expectedPods[i], node.AllocatedPod)
			}
		}
	})

	t.Run("DaemonSet pod scheduling with taints and tolerations", func(t *testing.T) {
		nodes := []shared.KubernetesNode{
			{Name: "node1", Taints: []v13.Taint{{Key: "key1", Value: "value1", Effect: v13.TaintEffectNoSchedule}}, MaxPodCount: 10},
			{Name: "node2", MaxPodCount: 10},
			{Name: "node3", Taints: []v13.Taint{{Key: "key2", Value: "value2", Effect: v13.TaintEffectNoSchedule}}, MaxPodCount: 10},
		}
		scheduler := New(nodes)

		tolerations := []v13.Toleration{
			{
				Key:      "key1",
				Operator: v13.TolerationOpEqual,
				Value:    "value1",
				Effect:   v13.TaintEffectNoSchedule,
			},
		}
		daemonSet := createDaemonSetWithTolerations("test-daemonset", tolerations)
		scheduler.AddDaemonSet(daemonSet)

		expectedPods := []int{1, 1, 0}
		for i, node := range scheduler.nodes {
			if node.AllocatedPod != expectedPods[i] {
				t.Errorf("Node %d should have %d DaemonSet pod(s), but has %d", i, expectedPods[i], node.AllocatedPod)
			}
		}
	})
}

package simulation

import (
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	v1 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setupSchedulerWithOneNode(cpu float64, memoryMB float64, maxPods int64) (*Scheduler, *shared.KubernetesNode) {
	node := shared.KubernetesNode{
		Name:        "test-node",
		VCores:      cpu,
		Memory:      memoryMB / 1024.0,
		MaxPodCount: maxPods,
	}
	scheduler := New([]shared.KubernetesNode{node})
	return scheduler, &scheduler.nodes[0]
}

func createPod(name string, cpuRequest float64, memoryRequestMB float64) v13.Pod {
	return v13.Pod{
		Spec: v13.PodSpec{
			Containers: []v13.Container{
				{
					Name: name,
					Resources: v13.ResourceRequirements{
						Requests: v13.ResourceList{
							v13.ResourceCPU:    *resource.NewMilliQuantity(int64(cpuRequest*1000), resource.DecimalSI),
							v13.ResourceMemory: *resource.NewQuantity(int64(memoryRequestMB*1024*1024), resource.BinarySI),
						},
					},
				},
			},
		},
	}
}

func createPodWithNodeAffinity(name string, key string, values []string) v13.Pod {
	return v13.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v13.PodSpec{
			Affinity: &v13.Affinity{
				NodeAffinity: &v13.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v13.NodeSelector{
						NodeSelectorTerms: []v13.NodeSelectorTerm{
							{
								MatchExpressions: []v13.NodeSelectorRequirement{
									{
										Key:      key,
										Operator: v13.NodeSelectorOpIn,
										Values:   values,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createPodWithPodAffinity(name string, key string, value string, topologyKey string) v13.Pod {
	return v13.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v13.PodSpec{
			Affinity: &v13.Affinity{
				PodAffinity: &v13.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v13.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{key: value},
							},
							TopologyKey: topologyKey,
						},
					},
				},
			},
		},
	}
}

func createPodWithPodAntiAffinity(name string, key string, value string, topologyKey string) v13.Pod {
	return v13.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v13.PodSpec{
			Affinity: &v13.Affinity{
				PodAntiAffinity: &v13.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v13.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{key: value},
							},
							TopologyKey: topologyKey,
						},
					},
				},
			},
		},
	}
}

func createPodWithLabels(name string, labels map[string]string) v13.Pod {
	return v13.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func createPodWithToleration(name string, key string, value string, effect v13.TaintEffect) v13.Pod {
	return v13.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v13.PodSpec{
			Tolerations: []v13.Toleration{
				{
					Key:      key,
					Operator: v13.TolerationOpEqual,
					Value:    value,
					Effect:   effect,
				},
			},
		},
	}
}

func createPodWithTolerationNoValue(name string, key string, effect v13.TaintEffect) v13.Pod {
	return v13.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v13.PodSpec{
			Tolerations: []v13.Toleration{
				{
					Key:      key,
					Operator: v13.TolerationOpExists,
					Effect:   effect,
				},
			},
		},
	}
}

func createDaemonSet(name string, nodeSelector map[string]string) v1.DaemonSet {
	return v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.DaemonSetSpec{
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					NodeSelector: nodeSelector,
				},
			},
		},
	}
}

func createDaemonSetWithAffinity(name string, affinity *v13.Affinity) v1.DaemonSet {
	return v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.DaemonSetSpec{
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					Affinity: affinity,
				},
			},
		},
	}
}

func createDaemonSetWithTolerations(name string, tolerations []v13.Toleration) v1.DaemonSet {
	return v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.DaemonSetSpec{
			Template: v13.PodTemplateSpec{
				Spec: v13.PodSpec{
					Tolerations: tolerations,
				},
			},
		},
	}
}

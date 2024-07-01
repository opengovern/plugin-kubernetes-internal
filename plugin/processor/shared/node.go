package shared

import corev1 "k8s.io/api/core/v1"

type KubernetesNode struct {
	Name         string
	VCores       float64
	Memory       float64
	MaxPodCount  int64
	Taints       []corev1.Taint
	Labels       map[string]string
	AllocatedCPU float64
	AllocatedMem float64
	AllocatedPod int
	Pods         []corev1.PodTemplateSpec
}

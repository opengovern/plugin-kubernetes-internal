package shared

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	labels2 "k8s.io/apimachinery/pkg/labels"
	"strings"
)

func LabelFilter(labelSelector string, labels map[string]string) bool {
	selector, err := labels2.Parse(labelSelector)
	if err != nil {
		panic(err)
	}

	return selector.Matches(labels2.Set(labels))
}

func PodsInNodes(pods []corev1.Pod, nodes []KubernetesNode) bool {
	for _, pod := range pods {
		var selectedNode *KubernetesNode
		for _, node := range nodes {
			if node.Name == pod.Spec.NodeName {
				selectedNode = &node
			}
		}

		if selectedNode != nil {
			return true
		}
	}
	return false
}

func NodeLabelFilter(nodeLabelSelector string, pods []corev1.Pod, nodes []corev1.Node) bool {
	fmt.Println(nodeLabelSelector, len(pods), len(nodes))
	nodeLabelSelector = strings.TrimSpace(nodeLabelSelector)
	if nodeLabelSelector == "" {
		return true
	}
	for _, pod := range pods {
		var selectedNode corev1.Node
		for _, node := range nodes {
			fmt.Println(node.Name, pod.Spec.NodeName)
			if node.Name == pod.Spec.NodeName {
				selectedNode = node
			}
		}

		if LabelFilter(nodeLabelSelector, selectedNode.Labels) {
			return true
		}
	}
	return false
}

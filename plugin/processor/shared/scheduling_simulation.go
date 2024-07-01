package shared

import (
	"fmt"
	"math"
	"sort"

	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	HeadroomFactor    = 0.85
	PodHeadroomFactor = 0.95
	GB                = 1024 * 1024 * 1024
)

type KubernetesNode struct {
	Name         string
	VCores       float64
	Memory       float64
	MaxPodCount  int
	Taints       []corev1.Taint
	Labels       map[string]string
	AllocatedCPU float64
	AllocatedMem float64
	AllocatedPod int
	Pods         []corev1.PodTemplateSpec
}

type Scheduler struct {
	nodes []KubernetesNode
	pdbs  []policyv1.PodDisruptionBudget
}

func New(nodes []KubernetesNode) *Scheduler {
	return &Scheduler{
		nodes: nodes,
	}
}

func (s *Scheduler) AddPodDisruptionBudget(pdb policyv1.PodDisruptionBudget) {
	s.pdbs = append(s.pdbs, pdb)
}

func (s *Scheduler) AddDaemonSet(item appv1.DaemonSet) bool {
	for i := range s.nodes {
		if s.canScheduleOnNode(item.Spec.Template.Spec, &s.nodes[i]) {
			s.schedulePod(item.Spec.Template, &s.nodes[i])
		}
	}
	return true
}

func (s *Scheduler) AddDeployment(item appv1.Deployment) bool {
	for i := 0; i < int(*item.Spec.Replicas); i++ {
		if !s.schedulePodWithStrategy(item.Spec.Template) {
			return false
		}
	}
	return true
}

func (s *Scheduler) AddJob(item batchv1.Job) bool {
	for i := 0; i < int(*item.Spec.Completions); i++ {
		if !s.schedulePodWithStrategy(item.Spec.Template) {
			return false
		}
	}
	return true
}

func (s *Scheduler) AddStatefulSet(item appv1.StatefulSet) bool {
	for i := 0; i < int(*item.Spec.Replicas); i++ {
		if !s.schedulePodWithStrategy(item.Spec.Template) {
			return false
		}
	}
	return true
}

func (s *Scheduler) AddPod(item corev1.Pod) bool {
	return s.schedulePodWithStrategy(corev1.PodTemplateSpec{
		ObjectMeta: item.ObjectMeta,
		Spec:       item.Spec,
	})
}

func (s *Scheduler) GetNodeUtilization() map[string]map[string]float64 {
	utilization := make(map[string]map[string]float64)
	for _, node := range s.nodes {
		utilization[node.Name] = map[string]float64{
			"CPU":    node.AllocatedCPU / node.VCores,
			"Memory": node.AllocatedMem / node.Memory,
			"Pods":   float64(node.AllocatedPod) / float64(node.MaxPodCount),
		}
	}
	return utilization
}

func (s *Scheduler) CanRemoveNode(nodeName string) (bool, error) {
	var nodeToRemove *KubernetesNode
	for i, node := range s.nodes {
		if node.Name == nodeName {
			nodeToRemove = &s.nodes[i]
			break
		}
	}

	if nodeToRemove == nil {
		return false, fmt.Errorf("node %s not found", nodeName)
	}

	// Check if the node is already empty
	if len(nodeToRemove.Pods) == 0 {
		return true, nil
	}

	// Create a temporary scheduler for simulation
	var tempNodes []KubernetesNode
	for _, node := range s.nodes {
		if node.Name != nodeName {
			tempNodes = append(tempNodes, node)
		}
	}
	tempScheduler := New(tempNodes)
	tempScheduler.pdbs = s.pdbs // Copy PodDisruptionBudgets

	// Simulate draining the node
	for _, pod := range nodeToRemove.Pods {
		if !s.canEvictPod(pod) {
			return false, nil
		}

		if !tempScheduler.schedulePodWithStrategy(pod) {
			return false, nil
		}
	}

	return true, nil
}

func (s *Scheduler) schedulePodWithStrategy(podSpec corev1.PodTemplateSpec) bool {
	// Sort nodes by most allocated resources
	sort.Slice(s.nodes, func(i, j int) bool {
		allocRatioI := max(s.nodes[i].AllocatedCPU/s.nodes[i].VCores, s.nodes[i].AllocatedMem/s.nodes[i].Memory)
		allocRatioJ := max(s.nodes[j].AllocatedCPU/s.nodes[j].VCores, s.nodes[j].AllocatedMem/s.nodes[j].Memory)
		return allocRatioI > allocRatioJ
	})

	// Try to schedule on the most allocated node that can accommodate the pod
	for i := range s.nodes {
		if s.canScheduleOnNode(podSpec.Spec, &s.nodes[i]) {
			s.schedulePod(podSpec, &s.nodes[i])
			return true
		}
	}

	return false
}

func (s *Scheduler) canScheduleOnNode(podSpec corev1.PodSpec, node *KubernetesNode) bool {
	// Check resources
	if !s.hasEnoughResources(podSpec, node) {
		return false
	}

	// Check taints and tolerations
	if !s.tolerates(podSpec, node.Taints) {
		return false
	}

	// Check node affinity
	if podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil {
		if !s.satisfiesNodeAffinity(podSpec.Affinity.NodeAffinity, node.Labels) {
			return false
		}
	}

	// Check pod affinity and anti-affinity
	if !s.satisfiesAffinityRules(podSpec, *node) {
		return false
	}

	// Check nodeSelector
	if podSpec.NodeSelector != nil {
		for key, value := range podSpec.NodeSelector {
			if nodeValue, exists := node.Labels[key]; !exists || nodeValue != value {
				return false
			}
		}
	}

	return true
}

func (s *Scheduler) satisfiesAffinityRules(podSpec corev1.PodSpec, node KubernetesNode) bool {
	if podSpec.Affinity == nil {
		return true
	}

	if podSpec.Affinity.PodAffinity != nil {
		for _, term := range podSpec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			if !s.satisfiesPodAffinityTerm(term, node) {
				return false
			}
		}
	}

	if podSpec.Affinity.PodAntiAffinity != nil {
		for _, term := range podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			if s.satisfiesPodAffinityTerm(term, node) {
				return false
			}
		}
	}

	return true
}

func (s *Scheduler) satisfiesPodAffinityTerm(term corev1.PodAffinityTerm, node KubernetesNode) bool {
	selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
	if err != nil {
		return false
	}

	for _, existingPod := range node.Pods {
		if selector.Matches(labels.Set(existingPod.Labels)) {
			// Check if the existing pod is in the specified topology key
			if term.TopologyKey != "" {
				if nodeValue, exists := node.Labels[term.TopologyKey]; exists {
					for _, otherNode := range s.nodes {
						if otherNodeValue, exists := otherNode.Labels[term.TopologyKey]; exists && otherNodeValue == nodeValue {
							return true
						}
					}
				}
			} else {
				return true
			}
		}
	}

	return false
}

func (s *Scheduler) hasEnoughResources(podSpec corev1.PodSpec, node *KubernetesNode) bool {
	cpuReq, memReq := s.getPodResourceRequests(podSpec)
	return node.AllocatedCPU+cpuReq <= node.VCores*HeadroomFactor &&
		node.AllocatedMem+memReq <= node.Memory*HeadroomFactor &&
		node.AllocatedPod+1 <= int(float64(node.MaxPodCount)*PodHeadroomFactor)
}

func (s *Scheduler) getPodResourceRequests(podSpec corev1.PodSpec) (float64, float64) {
	cpuReq, memReq := float64(0), float64(0)

	// Calculate for init containers
	for _, container := range podSpec.InitContainers {
		cpuReq = math.Max(cpuReq, float64(container.Resources.Requests.Cpu().MilliValue())/1000)
		memReq = math.Max(memReq, float64(container.Resources.Requests.Memory().Value())/(GB))
	}

	// Calculate for regular containers
	for _, container := range podSpec.Containers {
		cpuReq += float64(container.Resources.Requests.Cpu().MilliValue()) / 1000
		memReq += float64(container.Resources.Requests.Memory().Value()) / (GB)
	}

	return cpuReq, memReq
}

func (s *Scheduler) tolerates(podSpec corev1.PodSpec, nodeTaints []corev1.Taint) bool {
	for _, taint := range nodeTaints {
		if !taintTolerated(taint, podSpec.Tolerations) {
			return false
		}
	}
	return true
}

func taintTolerated(taint corev1.Taint, tolerations []corev1.Toleration) bool {
	for _, toleration := range tolerations {
		if (toleration.Key == taint.Key || toleration.Key == "") &&
			(toleration.Effect == taint.Effect || toleration.Effect == corev1.TaintEffectNoExecute) {
			return true
		}
	}
	return false
}
func (s *Scheduler) satisfiesNodeAffinity(nodeAffinity *corev1.NodeAffinity, nodeLabels map[string]string) bool {
	if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		for _, term := range nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			if s.matchNodeSelectorTerm(term, nodeLabels) {
				return true
			}
		}
		return false
	}
	return true
}

func (s *Scheduler) matchNodeSelectorTerm(term corev1.NodeSelectorTerm, nodeLabels map[string]string) bool {
	for _, expr := range term.MatchExpressions {
		if !s.matchNodeSelectorRequirement(expr, nodeLabels) {
			return false
		}
	}
	return true
}

func (s *Scheduler) matchNodeSelectorRequirement(req corev1.NodeSelectorRequirement, nodeLabels map[string]string) bool {
	switch req.Operator {
	case corev1.NodeSelectorOpIn:
		val, exists := nodeLabels[req.Key]
		return exists && contains(req.Values, val)
	case corev1.NodeSelectorOpNotIn:
		val, exists := nodeLabels[req.Key]
		return !exists || !contains(req.Values, val)
	case corev1.NodeSelectorOpExists:
		_, exists := nodeLabels[req.Key]
		return exists
	case corev1.NodeSelectorOpDoesNotExist:
		_, exists := nodeLabels[req.Key]
		return !exists
	default:
		return false
	}
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (s *Scheduler) canEvictPod(pod corev1.PodTemplateSpec) bool {
	for _, pdb := range s.pdbs {
		selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		if selector.Matches(labels.Set(pod.Labels)) {
			currentHealthy := s.countHealthyPods(pdb)
			if pdb.Spec.MinAvailable != nil {
				if currentHealthy <= pdb.Spec.MinAvailable.IntValue() {
					return false
				}
			} else if pdb.Spec.MaxUnavailable != nil {
				maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
				if currentHealthy-1 < s.getTotalPodCount(pdb)-maxUnavailable {
					return false
				}
			}
		}
	}
	return true
}

func (s *Scheduler) getTotalPodCount(pdb policyv1.PodDisruptionBudget) int {
	count := 0
	selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
	if err != nil {
		return 0
	}

	for _, node := range s.nodes {
		for _, pod := range node.Pods {
			if selector.Matches(labels.Set(pod.Labels)) {
				count++
			}
		}
	}
	return count
}

func (s *Scheduler) countHealthyPods(pdb policyv1.PodDisruptionBudget) int {
	count := 0
	selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
	if err != nil {
		return 0
	}

	for _, node := range s.nodes {
		for _, pod := range node.Pods {
			if selector.Matches(labels.Set(pod.Labels)) {
				count++
			}
		}
	}
	return count
}

func (s *Scheduler) schedulePod(pod corev1.PodTemplateSpec, node *KubernetesNode) {
	cpuReq, memReq := s.getPodResourceRequests(pod.Spec)
	node.AllocatedCPU += cpuReq
	node.AllocatedMem += memReq
	node.AllocatedPod++
	node.Pods = append(node.Pods, pod)
}

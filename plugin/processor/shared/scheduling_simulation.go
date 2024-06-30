package shared

import (
	"fmt"
	"math"
	"sort"

	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/batch/v1"
	v13 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	HeadroomFactor = 0.85
)

type KubernetesNode struct {
	Name         string
	VCores       float64
	Memory       float64
	MaxPodCount  int
	Taints       []v13.Taint
	Labels       map[string]string
	AllocatedCPU float64
	AllocatedMem float64
	AllocatedPod int
	Pods         []v13.PodTemplateSpec
}

type Scheduler struct {
	nodes              []KubernetesNode
	nodeCPUCapacity    map[string]float64
	nodeMemoryCapacity map[string]float64
	nodePodCapacity    map[string]int
	pdbs               []policyv1.PodDisruptionBudget
}

func New(nodes []KubernetesNode) *Scheduler {
	s := &Scheduler{
		nodes:              nodes,
		nodeCPUCapacity:    make(map[string]float64),
		nodeMemoryCapacity: make(map[string]float64),
		nodePodCapacity:    make(map[string]int),
	}
	for _, node := range nodes {
		s.nodeCPUCapacity[node.Name] = node.VCores * HeadroomFactor
		s.nodeMemoryCapacity[node.Name] = node.Memory * HeadroomFactor
		s.nodePodCapacity[node.Name] = int(float64(node.MaxPodCount) * HeadroomFactor)
	}
	return s
}

func (s *Scheduler) AddPodDisruptionBudget(pdb policyv1.PodDisruptionBudget) {
	s.pdbs = append(s.pdbs, pdb)
}

func (s *Scheduler) schedulePodWithStrategy(podSpec v13.PodTemplateSpec) bool {
	// Sort nodes by most allocated resources
	sort.Slice(s.nodes, func(i, j int) bool {
		allocRatioI := (s.nodes[i].AllocatedCPU / s.nodeCPUCapacity[s.nodes[i].Name]) + (s.nodes[i].AllocatedMem / s.nodeMemoryCapacity[s.nodes[i].Name])
		allocRatioJ := (s.nodes[j].AllocatedCPU / s.nodeCPUCapacity[s.nodes[j].Name]) + (s.nodes[j].AllocatedMem / s.nodeMemoryCapacity[s.nodes[j].Name])
		return allocRatioI > allocRatioJ
	})

	// Try to schedule on the most allocated node that can accommodate the pod
	for i := range s.nodes {
		if s.canScheduleOnNode(podSpec.Spec, s.nodes[i]) {
			s.schedulePod(podSpec, &s.nodes[i])
			return true
		}
	}

	return false
}

func (s *Scheduler) canScheduleOnNode(podSpec v13.PodSpec, node KubernetesNode) bool {
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
	if !s.satisfiesAffinityRules(podSpec, node) {
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

func (s *Scheduler) satisfiesAffinityRules(podSpec v13.PodSpec, node KubernetesNode) bool {
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

func (s *Scheduler) satisfiesPodAffinityTerm(term v13.PodAffinityTerm, node KubernetesNode) bool {
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

func (s *Scheduler) AddDaemonSet(item v1.DaemonSet) {
	for _, node := range s.nodes {
		if s.canScheduleOnNode(item.Spec.Template.Spec, node) {
			s.schedulePod(item.Spec.Template, &node)
		}
	}
}

func (s *Scheduler) AddDeployment(item v1.Deployment) {
	for i := 0; i < int(*item.Spec.Replicas); i++ {
		s.schedulePodWithStrategy(item.Spec.Template)
	}
}

func (s *Scheduler) AddJob(item v12.Job) {
	for i := 0; i < int(*item.Spec.Completions); i++ {
		s.schedulePodWithStrategy(item.Spec.Template)
	}
}

func (s *Scheduler) AddStatefulSet(item v1.StatefulSet) {
	for i := 0; i < int(*item.Spec.Replicas); i++ {
		s.schedulePodWithStrategy(item.Spec.Template)
	}
}

func (s *Scheduler) AddPod(item v13.Pod) {
	s.schedulePodWithStrategy(v13.PodTemplateSpec{
		ObjectMeta: item.ObjectMeta,
		Spec:       item.Spec,
	})
}

func (s *Scheduler) hasEnoughResources(podSpec v13.PodSpec, node KubernetesNode) bool {
	cpuReq, memReq := s.getPodResourceRequests(podSpec)
	return node.AllocatedCPU+cpuReq <= s.nodeCPUCapacity[node.Name] &&
		node.AllocatedMem+memReq <= s.nodeMemoryCapacity[node.Name] &&
		node.AllocatedPod+1 <= s.nodePodCapacity[node.Name]
}

func (s *Scheduler) getPodResourceRequests(podSpec v13.PodSpec) (float64, float64) {
	cpuReq, memReq := float64(0), float64(0)

	// Calculate for init containers
	for _, container := range podSpec.InitContainers {
		cpuReq = math.Max(cpuReq, float64(container.Resources.Requests.Cpu().MilliValue())/1000)
		memReq = math.Max(memReq, float64(container.Resources.Requests.Memory().Value())/(1024*1024*1024))
	}

	// Calculate for regular containers
	for _, container := range podSpec.Containers {
		cpuReq += float64(container.Resources.Requests.Cpu().MilliValue()) / 1000
		memReq += float64(container.Resources.Requests.Memory().Value()) / (1024 * 1024 * 1024)
	}

	return cpuReq, memReq
}

func (s *Scheduler) tolerates(podSpec v13.PodSpec, nodeTaints []v13.Taint) bool {
	for _, taint := range nodeTaints {
		if !taintTolerated(taint, podSpec.Tolerations) {
			return false
		}
	}
	return true
}

func taintTolerated(taint v13.Taint, tolerations []v13.Toleration) bool {
	for _, toleration := range tolerations {
		if (toleration.Key == taint.Key || toleration.Key == "") &&
			(toleration.Effect == taint.Effect || toleration.Effect == v13.TaintEffectNoExecute) {
			return true
		}
	}
	return false
}
func (s *Scheduler) satisfiesNodeAffinity(nodeAffinity *v13.NodeAffinity, nodeLabels map[string]string) bool {
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

func (s *Scheduler) matchNodeSelectorTerm(term v13.NodeSelectorTerm, nodeLabels map[string]string) bool {
	for _, expr := range term.MatchExpressions {
		if !s.matchNodeSelectorRequirement(expr, nodeLabels) {
			return false
		}
	}
	return true
}

func (s *Scheduler) matchNodeSelectorRequirement(req v13.NodeSelectorRequirement, nodeLabels map[string]string) bool {
	switch req.Operator {
	case v13.NodeSelectorOpIn:
		val, exists := nodeLabels[req.Key]
		return exists && contains(req.Values, val)
	case v13.NodeSelectorOpNotIn:
		val, exists := nodeLabels[req.Key]
		return !exists || !contains(req.Values, val)
	case v13.NodeSelectorOpExists:
		_, exists := nodeLabels[req.Key]
		return exists
	case v13.NodeSelectorOpDoesNotExist:
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

func (s *Scheduler) GetNodeUtilization() map[string]map[string]float64 {
	utilization := make(map[string]map[string]float64)
	for _, node := range s.nodes {
		utilization[node.Name] = map[string]float64{
			"CPU":    node.AllocatedCPU / s.nodeCPUCapacity[node.Name],
			"Memory": node.AllocatedMem / s.nodeMemoryCapacity[node.Name],
			"Pods":   float64(node.AllocatedPod) / float64(s.nodePodCapacity[node.Name]),
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
	tempNodes := make([]KubernetesNode, 0, len(s.nodes)-1)
	for _, node := range s.nodes {
		if node.Name != nodeName {
			tempNodes = append(tempNodes, node)
		}
	}
	tempScheduler := New(tempNodes)
	tempScheduler.pdbs = s.pdbs // Copy PodDisruptionBudgets

	// Simulate draining the node
	for _, pod := range nodeToRemove.Pods {
		if !tempScheduler.canEvictPod(pod) {
			return false, nil
		}

		if !tempScheduler.schedulePodWithStrategy(pod) {
			return false, nil
		}
	}

	return true, nil
}

func (s *Scheduler) canEvictPod(pod v13.PodTemplateSpec) bool {
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

func (s *Scheduler) schedulePod(pod v13.PodTemplateSpec, node *KubernetesNode) {
	cpuReq, memReq := s.getPodResourceRequests(pod.Spec)
	node.AllocatedCPU += cpuReq
	node.AllocatedMem += memReq
	node.AllocatedPod++
	node.Pods = append(node.Pods, pod)
}

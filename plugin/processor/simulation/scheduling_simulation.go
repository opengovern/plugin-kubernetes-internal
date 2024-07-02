package simulation

import (
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"math"
	"sort"
	"strings"

	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	CPUHeadroomFactor    = 0.85
	MemoryHeadroomFactor = 0.85
	PodHeadroomFactor    = 0.95
)

const (
	GB = 1024 * 1024 * 1024
)

type Scheduler struct {
	nodes []shared.KubernetesNode
	pdbs  []policyv1.PodDisruptionBudget
}

func New(nodes []shared.KubernetesNode) *Scheduler {
	return &Scheduler{
		nodes: nodes,
	}
}

func (s *Scheduler) AddPodDisruptionBudget(pdb policyv1.PodDisruptionBudget) {
	s.pdbs = append(s.pdbs, pdb)
}

func (s *Scheduler) AddDaemonSet(item appv1.DaemonSet) (bool, string) {
	reasonCount := map[string]int{}
	for i := range s.nodes {
		if ok, reason := s.canScheduleOnNode(item.Spec.Template.Spec, &s.nodes[i]); ok {
			s.schedulePod(item.Spec.Template, &s.nodes[i])
		} else {
			reasonCount[reason]++
		}
	}

	var reasons []string
	for r, c := range reasonCount {
		reasons = append(reasons, fmt.Sprintf("%s on %d nodes", r, c))
	}
	reason := ""
	if len(reasons) > 0 {
		reason = fmt.Sprintf("failed to schedule due to: %s", strings.Join(reasons, ","))
	}

	return true, reason
}

func (s *Scheduler) AddDeployment(item appv1.Deployment) (bool, string) {
	for i := 0; i < int(*item.Spec.Replicas); i++ {
		if ok, reason := s.schedulePodWithStrategy(item.Spec.Template); !ok {
			return false, reason
		}
	}
	return true, ""
}

func (s *Scheduler) AddJob(item batchv1.Job) (bool, string) {
	for i := 0; i < int(*item.Spec.Completions); i++ {
		if ok, reason := s.schedulePodWithStrategy(item.Spec.Template); !ok {
			return false, reason
		}
	}
	return true, ""
}

func (s *Scheduler) AddStatefulSet(item appv1.StatefulSet) (bool, string) {
	for i := 0; i < int(*item.Spec.Replicas); i++ {
		if ok, reason := s.schedulePodWithStrategy(item.Spec.Template); !ok {
			return false, reason
		}
	}
	return true, ""
}

func (s *Scheduler) AddPod(item corev1.Pod) (bool, string) {
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
	var nodeToRemove *shared.KubernetesNode
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
	var tempNodes []shared.KubernetesNode
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

		if ok, _ := tempScheduler.schedulePodWithStrategy(pod); !ok {
			return false, nil
		}
	}

	return true, nil
}

func (s *Scheduler) schedulePodWithStrategy(podSpec corev1.PodTemplateSpec) (bool, string) {
	// Sort nodes by most allocated resources
	sort.Slice(s.nodes, func(i, j int) bool {
		allocRatioI := max(s.nodes[i].AllocatedCPU/s.nodes[i].VCores, s.nodes[i].AllocatedMem/s.nodes[i].Memory)
		allocRatioJ := max(s.nodes[j].AllocatedCPU/s.nodes[j].VCores, s.nodes[j].AllocatedMem/s.nodes[j].Memory)
		return allocRatioI > allocRatioJ
	})

	// Try to schedule on the most allocated node that can accommodate the pod
	reasonCount := map[string]int{}
	for i := range s.nodes {
		if ok, reason := s.canScheduleOnNode(podSpec.Spec, &s.nodes[i]); ok {
			s.schedulePod(podSpec, &s.nodes[i])
			return true, ""
		} else {
			reasonCount[reason]++
		}
	}

	var reasons []string
	for r, c := range reasonCount {
		reasons = append(reasons, fmt.Sprintf("%s on %d nodes", r, c))
	}

	return false, fmt.Sprintf("failed to schedule due to: %s", strings.Join(reasons, ","))
}

const (
	SchedulingReason_NotEnoughCPU               = "not enough cpu"
	SchedulingReason_NotEnoughMemory            = "not enough memory"
	SchedulingReason_NotEnoughPod               = "not enough pods"
	SchedulingReason_NotTolerated               = "not tolerated"
	SchedulingReason_NodeAffinityNotSatisfied   = "node affinity not satisfied"
	SchedulingReason_AffinityNotSatisfied       = "affinity not satisfied"
	SchedulingReason_NodeSelectorLabelMismatch  = "node selector label mismatch"
	SchedulingReason_NodeSelectorLabelNotExists = "node selector label not exists"
)

func (s *Scheduler) canScheduleOnNode(podSpec corev1.PodSpec, node *shared.KubernetesNode) (bool, string) {
	// Check resources
	if ok, reason := s.hasEnoughResources(podSpec, node); !ok {
		return false, reason
	}

	// Check taints and tolerations
	if !s.tolerates(podSpec, node.Taints) {
		return false, SchedulingReason_NotTolerated
	}

	// Check node affinity
	if podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil {
		if !s.satisfiesNodeAffinity(podSpec.Affinity.NodeAffinity, node.Labels) {
			return false, SchedulingReason_NodeAffinityNotSatisfied
		}
	}

	// Check pod affinity and anti-affinity
	if !s.satisfiesAffinityRules(podSpec, *node) {
		return false, SchedulingReason_AffinityNotSatisfied
	}

	// Check nodeSelector
	if podSpec.NodeSelector != nil {
		for key, value := range podSpec.NodeSelector {
			nodeValue, exists := node.Labels[key]
			if !exists {
				fmt.Println("node labels", node)
				return false, SchedulingReason_NodeSelectorLabelNotExists
			}
			if nodeValue != value {
				return false, SchedulingReason_NodeSelectorLabelMismatch
			}
		}
	}

	return true, ""
}

func (s *Scheduler) satisfiesAffinityRules(podSpec corev1.PodSpec, node shared.KubernetesNode) bool {
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

func (s *Scheduler) satisfiesPodAffinityTerm(term corev1.PodAffinityTerm, node shared.KubernetesNode) bool {
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

func (s *Scheduler) hasEnoughResources(podSpec corev1.PodSpec, node *shared.KubernetesNode) (bool, string) {
	cpuReq, memReq := s.getPodResourceRequests(podSpec)

	if node.AllocatedCPU+cpuReq > node.VCores*CPUHeadroomFactor {
		return false, SchedulingReason_NotEnoughCPU
	}
	if node.AllocatedMem+memReq > node.Memory*MemoryHeadroomFactor {
		return false, SchedulingReason_NotEnoughMemory
	}
	if node.AllocatedPod+1 > int(float64(node.MaxPodCount)*PodHeadroomFactor) {
		return false, SchedulingReason_NotEnoughPod
	}

	return true, ""
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

func (s *Scheduler) schedulePod(pod corev1.PodTemplateSpec, node *shared.KubernetesNode) {
	cpuReq, memReq := s.getPodResourceRequests(pod.Spec)
	node.AllocatedCPU += cpuReq
	node.AllocatedMem += memReq
	node.AllocatedPod++
	node.Pods = append(node.Pods, pod)
}

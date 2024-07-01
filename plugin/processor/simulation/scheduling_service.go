package simulation

import (
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/daemonsets"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/deployments"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/jobs"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/pods"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/statefulsets"
	policyv1 "k8s.io/api/policy/v1"
)

type SchedulerService struct {
	nodes        []shared.KubernetesNode
	pdbs         []policyv1.PodDisruptionBudget
	daemonSets   map[string]daemonsets.DaemonsetItem
	deployments  map[string]deployments.DeploymentItem
	jobs         map[string]jobs.JobItem
	statefulsets map[string]statefulsets.StatefulsetItem
	pods         map[string]pods.PodItem
}

func NewSchedulerService(nodes []shared.KubernetesNode) *SchedulerService {
	return &SchedulerService{
		nodes:        nodes,
		pdbs:         nil,
		daemonSets:   map[string]daemonsets.DaemonsetItem{},
		deployments:  map[string]deployments.DeploymentItem{},
		jobs:         map[string]jobs.JobItem{},
		statefulsets: map[string]statefulsets.StatefulsetItem{},
		pods:         map[string]pods.PodItem{},
	}
}

func (s *SchedulerService) AddPodDisruptionBudget(pdb policyv1.PodDisruptionBudget) {
	s.pdbs = append(s.pdbs, pdb)
}

func (s *SchedulerService) AddDaemonSet(item daemonsets.DaemonsetItem) {
	s.daemonSets[item.GetID()] = item
}

func (s *SchedulerService) AddDeployment(item deployments.DeploymentItem) {
	s.deployments[item.GetID()] = item
}

func (s *SchedulerService) AddJob(item jobs.JobItem) {
	s.jobs[item.GetID()] = item
}

func (s *SchedulerService) AddStatefulSet(item statefulsets.StatefulsetItem) {
	s.statefulsets[item.GetID()] = item
}

func (s *SchedulerService) AddPod(item pods.PodItem) {
	s.pods[item.GetID()] = item
}

func (s *SchedulerService) Simulate() ([]shared.KubernetesNode, error) {
	var nodes []shared.KubernetesNode
	for _, n := range s.nodes {
		nodes = append(nodes, n)
	}

	return s.simulate(nodes)
}

func (s *SchedulerService) simulate(nodes []shared.KubernetesNode) ([]shared.KubernetesNode, error) {
	if len(nodes) <= 1 {
		return nil, nil
	}

	scheduler := New(nodes)
	for _, pb := range s.pdbs {
		scheduler.AddPodDisruptionBudget(pb)
	}

	for _, r := range s.daemonSets {
		if !scheduler.AddDaemonSet(r.Daemonset) {
			return nil, fmt.Errorf("failed to add daemonSet %s", r.GetID())
		}
	}
	for _, r := range s.deployments {
		if !scheduler.AddDeployment(r.Deployment) {
			return nil, fmt.Errorf("failed to add deployment %s", r.GetID())
		}
	}
	for _, r := range s.jobs {
		if !scheduler.AddJob(r.Job) {
			return nil, fmt.Errorf("failed to add job %s", r.GetID())
		}
	}
	for _, r := range s.statefulsets {
		if !scheduler.AddStatefulSet(r.Statefulset) {
			return nil, fmt.Errorf("failed to add statefulset %s", r.GetID())
		}
	}
	for _, r := range s.pods {
		if !scheduler.AddPod(r.Pod) {
			return nil, fmt.Errorf("failed to add pod %s", r.GetID())
		}
	}

	var removed []shared.KubernetesNode
	var remaining []shared.KubernetesNode
	for _, n := range nodes {
		ok := false

		if len(removed) == 0 {
			var err error
			ok, err = scheduler.CanRemoveNode(n.Name)
			if err != nil {
				return nil, err
			}
		}

		if ok {
			removed = append(removed, n)
		} else {
			remaining = append(remaining, n)
		}
	}

	if len(removed) == 0 {
		return removed, nil
	}

	res, err := s.simulate(remaining)
	if err != nil {
		return nil, err
	}

	return append(removed, res...), nil
}

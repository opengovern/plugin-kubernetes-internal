package simulation

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

type SchedulerService struct {
	nodes        []shared.KubernetesNode
	pdbs         []policyv1.PodDisruptionBudget
	daemonSets   utils.ConcurrentMap[string, appv1.DaemonSet]
	deployments  utils.ConcurrentMap[string, appv1.Deployment]
	jobs         utils.ConcurrentMap[string, v1.Job]
	statefulsets utils.ConcurrentMap[string, appv1.StatefulSet]
	pods         utils.ConcurrentMap[string, corev1.Pod]
}

func NewSchedulerService(nodes []shared.KubernetesNode) *SchedulerService {
	return &SchedulerService{
		nodes:        nodes,
		pdbs:         nil,
		daemonSets:   utils.NewConcurrentMap[string, appv1.DaemonSet](),
		deployments:  utils.NewConcurrentMap[string, appv1.Deployment](),
		jobs:         utils.NewConcurrentMap[string, v1.Job](),
		statefulsets: utils.NewConcurrentMap[string, appv1.StatefulSet](),
		pods:         utils.NewConcurrentMap[string, corev1.Pod](),
	}
}

func (s *SchedulerService) AddPodDisruptionBudget(pdb policyv1.PodDisruptionBudget) {
	s.pdbs = append(s.pdbs, pdb)
}

func (s *SchedulerService) AddDaemonSet(item appv1.DaemonSet) {
	s.daemonSets.Set(fmt.Sprintf("appv1.DaemonSet/%s/%s", item.Namespace, item.Name), item)
}

func (s *SchedulerService) AddDeployment(item appv1.Deployment) {
	s.deployments.Set(fmt.Sprintf("appv1.Deployment/%s/%s", item.Namespace, item.Name), item)
}

func (s *SchedulerService) AddJob(item v1.Job) {
	s.jobs.Set(fmt.Sprintf("v1.Job/%s/%s", item.Namespace, item.Name), item)
}

func (s *SchedulerService) AddStatefulSet(item appv1.StatefulSet) {
	s.statefulsets.Set(fmt.Sprintf("appv1.StatefulSet/%s/%s", item.Namespace, item.Name), item)
}

func (s *SchedulerService) AddPod(item corev1.Pod) {
	s.pods.Set(fmt.Sprintf("corev1.Pod/%s/%s", item.Namespace, item.Name), item)
}

func (s *SchedulerService) Simulate() ([]shared.KubernetesNode, error) {
	var nodes []shared.KubernetesNode
	for _, n := range s.nodes {
		nodes = append(nodes, n)
	}

	return s.simulate(nodes)
}

const (
	ResourcePriority_PodAffinity  = "podAffinity"
	ResourcePriority_NodeAffinity = "nodeAffinity"
	ResourcePriority_Toleration   = "toleration"
	ResourcePriority_NodeSelector = "nodeSelector"
	ResourcePriority_None         = "none"
)

func resourcePriority(podSpec corev1.PodSpec) string {
	if podSpec.Affinity != nil {
		if podSpec.Affinity.PodAffinity != nil || podSpec.Affinity.PodAntiAffinity != nil {
			return ResourcePriority_PodAffinity
		} else if podSpec.Affinity.NodeAffinity != nil {
			return ResourcePriority_NodeAffinity
		}
	}

	if len(podSpec.Tolerations) > 0 {
		return ResourcePriority_Toleration
	}

	if len(podSpec.NodeSelector) > 0 {
		return ResourcePriority_NodeSelector
	}
	return ResourcePriority_None
}

func (s *SchedulerService) simulate(nodes []shared.KubernetesNode) ([]shared.KubernetesNode, error) {
	if len(nodes) <= 1 {
		return nil, nil
	}

	scheduler := New(nodes)
	for _, pb := range s.pdbs {
		scheduler.AddPodDisruptionBudget(pb)
	}

	for _, priority := range []string{ResourcePriority_PodAffinity, ResourcePriority_NodeAffinity,
		ResourcePriority_Toleration, ResourcePriority_NodeSelector, ResourcePriority_None} {
		var err error
		s.daemonSets.Range(func(_ string, r appv1.DaemonSet) bool {
			if resourcePriority(r.Spec.Template.Spec) != priority {
				return true
			}

			if ok, reason := scheduler.AddDaemonSet(r); !ok {
				err = fmt.Errorf("failed to add daemonSet %s due to: %s", r.Name+"/"+r.Namespace, reason)
			}
			return true
		})
		if err != nil {
			return nil, err
		}

		s.deployments.Range(func(_ string, r appv1.Deployment) bool {
			if resourcePriority(r.Spec.Template.Spec) != priority {
				return true
			}

			if ok, reason := scheduler.AddDeployment(r); !ok {
				err = fmt.Errorf("failed to add deployment %s due to: %s", r.Name+"/"+r.Namespace, reason)
			}
			return true
		})
		if err != nil {
			return nil, err
		}

		s.jobs.Range(func(_ string, r v1.Job) bool {
			if resourcePriority(r.Spec.Template.Spec) != priority {
				return true
			}

			if ok, reason := scheduler.AddJob(r); !ok {
				err = fmt.Errorf("failed to add job %s due to: %s", r.Name+"/"+r.Namespace, reason)
			}
			return true
		})
		if err != nil {
			return nil, err
		}

		s.statefulsets.Range(func(_ string, r appv1.StatefulSet) bool {
			if resourcePriority(r.Spec.Template.Spec) != priority {
				return true
			}

			if ok, reason := scheduler.AddStatefulSet(r); !ok {
				err = fmt.Errorf("failed to add statefulset %s due to: %s", r.Name+"/"+r.Namespace, reason)
			}
			return true
		})
		if err != nil {
			return nil, err
		}

		s.pods.Range(func(_ string, r corev1.Pod) bool {
			if resourcePriority(r.Spec) != priority {
				return true
			}

			if ok, reason := scheduler.AddPod(r); !ok {
				err = fmt.Errorf("failed to add pod %s due to: %s", r.Name+"/"+r.Namespace, reason)
			}
			return true
		})
		if err != nil {
			return nil, err
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

	for idx, r := range remaining {
		r.AllocatedCPU = 0
		r.AllocatedMem = 0
		r.AllocatedPod = 0
		remaining[idx] = r
	}
	res, err := s.simulate(remaining)
	if err != nil {
		return nil, err
	}

	return append(removed, res...), nil
}

func (s *SchedulerService) SetNodes(knodes []shared.KubernetesNode) {
	s.nodes = knodes
}

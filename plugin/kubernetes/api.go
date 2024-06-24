package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/utils"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"sort"
	"strings"
	"sync"
	"time"
)

var KaytuNotFoundErr = errors.New("kaytu not found")

type Kubernetes struct {
	restClientCfg *restclient.Config
	kubeCfg       *api.Config
	clientset     *kubernetes.Clientset
}

func NewKubernetes(cfg *restclient.Config, kubeCfg *api.Config) (*Kubernetes, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Kubernetes{restClientCfg: cfg, kubeCfg: kubeCfg, clientset: clientset}, nil
}

func (s *Kubernetes) Identify() map[string]string {
	result := make(map[string]string)

	if s.restClientCfg != nil {
		result["cluster_server"] = utils.HashString(s.restClientCfg.Host)
	}

	if s.kubeCfg == nil {
		return result
	}

	currentContext := s.kubeCfg.Contexts[s.kubeCfg.CurrentContext]
	if currentContext == nil && len(s.kubeCfg.Contexts) > 1 {
		return result
	} else if currentContext == nil {
		for _, v := range s.kubeCfg.Contexts {
			currentContext = v
		}
	}
	result["context_name"] = s.kubeCfg.CurrentContext
	result["cluster_name"] = currentContext.Cluster
	result["auth_info_name"] = currentContext.AuthInfo

	currentCluster := s.kubeCfg.Clusters[currentContext.Cluster]
	if currentCluster == nil {
		return result
	}
	result["cluster_server"] = utils.HashString(currentCluster.Server)

	return result
}

func (s *Kubernetes) ListAllNamespaces(ctx context.Context) ([]corev1.Namespace, error) {
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return namespaces.Items, nil
}

func (s *Kubernetes) ListPodsInNamespace(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	pods, err := s.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func (s *Kubernetes) ListDeploymentsInNamespace(ctx context.Context, namespace string) ([]appv1.Deployment, error) {
	deployments, err := s.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return deployments.Items, nil
}

func (s *Kubernetes) ListStatefulsetsInNamespace(ctx context.Context, namespace string) ([]appv1.StatefulSet, error) {
	statefulsets, err := s.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return statefulsets.Items, nil
}

func (s *Kubernetes) ListDaemonsetsInNamespace(ctx context.Context, namespace string) ([]appv1.DaemonSet, error) {
	daemonsets, err := s.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return daemonsets.Items, nil
}

func (s *Kubernetes) ListJobsInNamespace(ctx context.Context, namespace string) ([]batchv1.Job, error) {
	jobs, err := s.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return jobs.Items, nil
}

func (s *Kubernetes) ListDeploymentPodsAndHistoricalReplicaSets(ctx context.Context, deployment appv1.Deployment, maxDays int) ([]corev1.Pod, []string, error) {
	timeCut := time.Now().AddDate(0, 0, -maxDays).Truncate(24 * time.Hour)

	probableReplicaSets, err := s.clientset.AppsV1().ReplicaSets(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, nil, err
	}
	var historicalReplicaSets []appv1.ReplicaSet
	var activeReplicaSets []appv1.ReplicaSet
	for _, rs := range probableReplicaSets.Items {
		isOwnedByDeployment := false
		for _, owner := range rs.ObjectMeta.OwnerReferences {
			if owner.UID == deployment.UID {
				isOwnedByDeployment = true
				break
			}
		}
		if !isOwnedByDeployment {
			continue
		}
		rs := rs
		if rs.Status.Replicas == deployment.Status.Replicas {
			activeReplicaSets = append(activeReplicaSets, rs)
		} else {
			historicalReplicaSets = append(historicalReplicaSets, rs)
		}
	}
	sort.Slice(activeReplicaSets, func(i, j int) bool {
		return activeReplicaSets[j].CreationTimestamp.Before(&activeReplicaSets[i].CreationTimestamp)
	})
	var activeReplicaSet *appv1.ReplicaSet
	if len(activeReplicaSets) > 0 {
		activeReplicaSet = &activeReplicaSets[0]
	}
	if activeReplicaSet == nil {
		return nil, nil, errors.New("no active replica set found for deployment")
	}

	if len(activeReplicaSets) > 1 {
		historicalReplicaSets = append(historicalReplicaSets, activeReplicaSets[1:]...)
	}
	sort.Slice(historicalReplicaSets, func(i, j int) bool {
		return historicalReplicaSets[j].CreationTimestamp.Before(&historicalReplicaSets[i].CreationTimestamp)
	})
	historicalReplicaSetNames := make([]string, 0, len(historicalReplicaSets))
	for i, rs := range historicalReplicaSets {
		// ignore the replica sets that are older than the time cut
		if rs.CreationTimestamp.Before(&metav1.Time{Time: timeCut}) {
			continue
		}

		// add the last replicaset before the time cut to the list to make sure we have the within the time cut
		//                ↓ time cut
		// time ----------|---------------------
		// rs   <---><-------><------><-------->
		//             ↑ last rs before time cut
		if len(historicalReplicaSets) == 0 && i > 0 {
			historicalReplicaSetNames = append(historicalReplicaSetNames, activeReplicaSets[i-1].Name)
		}
		historicalReplicaSetNames = append(historicalReplicaSetNames, rs.Name)
	}

	probablePods, err := s.clientset.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, nil, err
	}

	pods := make([]corev1.Pod, 0, len(probablePods.Items))
	for _, pod := range probablePods.Items {
		isOwnedByReplicaSet := false
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.UID == activeReplicaSet.UID {
				isOwnedByReplicaSet = true
				break
			}
		}
		if !isOwnedByReplicaSet || pod.Status.Phase != corev1.PodRunning {
			continue
		}
		pods = append(pods, pod)
	}

	return pods, historicalReplicaSetNames, nil
}

func (s *Kubernetes) ListStatefulsetPods(ctx context.Context, statefulset appv1.StatefulSet) ([]corev1.Pod, error) {
	probablePods, err := s.clientset.CoreV1().Pods(statefulset.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(statefulset.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(probablePods.Items))
	for _, pod := range probablePods.Items {
		isOwnedByStatefulset := false
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.UID == statefulset.UID {
				isOwnedByStatefulset = true
				break
			}
		}
		if !isOwnedByStatefulset || pod.Status.Phase != corev1.PodRunning {
			continue
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func (s *Kubernetes) ListDaemonsetPods(ctx context.Context, daemonset appv1.DaemonSet) ([]corev1.Pod, error) {
	probablePods, err := s.clientset.CoreV1().Pods(daemonset.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(daemonset.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(probablePods.Items))
	for _, pod := range probablePods.Items {
		isOwnedByDaemonset := false
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.UID == daemonset.UID {
				isOwnedByDaemonset = true
				break
			}
		}
		if !isOwnedByDaemonset || pod.Status.Phase != corev1.PodRunning {
			continue
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func (s *Kubernetes) ListJobPods(ctx context.Context, job batchv1.Job) ([]corev1.Pod, error) {
	probablePods, err := s.clientset.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(job.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(probablePods.Items))
	for _, pod := range probablePods.Items {
		isOwnedByJob := false
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.UID == job.UID {
				isOwnedByJob = true
				break
			}
		}
		if !isOwnedByJob || pod.Status.Phase != corev1.PodRunning {
			continue
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func (s *Kubernetes) DiscoverAndPortForwardKaytuAgent(ctx context.Context, reconnectMutex *sync.Mutex) (chan struct{}, string, error) {
	stopChan := make(chan struct{}, 1)

	port, err := getNextOpenPort()
	if err != nil {
		return nil, "", err
	}

	svc, err := s.findKaytuService(ctx)
	if err != nil {
		return nil, "", err
	}
	if svc == nil {
		return nil, "", KaytuNotFoundErr
	}

	err = s.portForward(ctx, svc.Namespace, svc.Name, []string{fmt.Sprintf("%d:8001", port)}, reconnectMutex, stopChan)
	if err != nil {
		return nil, "", err
	}

	return stopChan, fmt.Sprintf("localhost:%d", port), nil
}

func (s *Kubernetes) findKaytuService(ctx context.Context) (*corev1.Service, error) {
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, namespace := range namespaces.Items {
		svcs, err := s.clientset.CoreV1().Services(namespace.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, svc := range svcs.Items {
			if strings.Contains(svc.Name, "kaytu-agent") {
				for _, port := range svc.Spec.Ports {
					if port.Port == 8001 {
						return &svc, nil
					}
				}
			}
		}
	}
	return nil, nil
}

package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/utils"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

type Kubernetes struct {
	restClientCfg *restclient.Config
	kubeCfg       *api.Config
	clientset     *kubernetes.Clientset
	stopChan      chan struct{}
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
	currentContext := s.kubeCfg.Contexts[s.kubeCfg.CurrentContext]
	if currentContext == nil {
		return result
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

func (s *Kubernetes) ListDeploymentPods(ctx context.Context, deployment appv1.Deployment) ([]corev1.Pod, error) {
	probableReplicaSets, err := s.clientset.AppsV1().ReplicaSets(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}
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
			break
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
		return nil, errors.New("no active replica set found for deployment")
	}

	probablePods, err := s.clientset.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return nil, err
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

	return pods, nil
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

func (s *Kubernetes) DiscoverPrometheus(ctx context.Context, reconnectMutex *sync.Mutex) (chan struct{}, error) {
	svc, err := s.findPrometheusService(ctx)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("prometheus not found")
	}

	err = s.portForward(ctx, svc.Namespace, svc.Name, []string{"9090:9090"}, reconnectMutex)
	if err != nil {
		return nil, err
	}

	return s.stopChan, nil
}

func (s *Kubernetes) findPrometheusService(ctx context.Context) (*corev1.Service, error) {
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
			if strings.Contains(svc.Name, "prometheus") {
				for _, port := range svc.Spec.Ports {
					if port.Port == 9090 {
						return &svc, nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (s *Kubernetes) portForward(ctx context.Context, namespace, serviceName string, ports []string, mutex *sync.Mutex) error {
	service, err := s.clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	pods, err := s.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(service.Spec.Selector).AsSelector().String(),
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return errors.New("no pods found for service selector")
	}
	podName := pods.Items[0].Name

	roundTripper, upgrader, err := spdy.RoundTripperFor(s.restClientCfg)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP, err := url.Parse(s.restClientCfg.Host)
	if err != nil {
		return err
	}

	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP.Host}
	fmt.Println("prometheus - connecting")
	s.reconnect(upgrader, ports, roundTripper, serverURL, mutex)

	return nil
}

func (s *Kubernetes) reconnect(upgrader spdy.Upgrader, ports []string, roundTripper http.RoundTripper, serverURL url.URL, mutex *sync.Mutex) {
	fmt.Println("prometheus - reconnect")
	readyChan := make(chan struct{}, 1)
	s.stopChan = make(chan struct{}, 1)

	mutex.Lock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("prometheus - ", r)
			}

			s.reconnect(upgrader, ports, roundTripper, serverURL, mutex)
			return
		}()

		out, errOut := new(bytes.Buffer), new(bytes.Buffer)

		dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, "POST", &serverURL)

		forwarder, err := portforward.New(dialer, ports, s.stopChan, readyChan, out, errOut)
		if err != nil {
			fmt.Println("prometheus - ", err)
			return
		}

		err = forwarder.ForwardPorts()
		if err != nil {
			fmt.Println("prometheus - ", err)
			return
		}
	}()
	<-readyChan // This line will block until the port forwarding is ready to get traffic.
	mutex.Unlock()
}

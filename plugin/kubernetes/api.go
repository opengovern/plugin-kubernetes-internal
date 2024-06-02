package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/utils"
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

func (s *Kubernetes) DiscoverPrometheus(ctx context.Context, reconnectMutex *sync.Mutex) (chan struct{}, error) {
	svc, err := s.findPrometheusService(ctx)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("prometheus not found")
	}

	return s.portForward(ctx, svc.Namespace, svc.Name, []string{"9090:9090"}, reconnectMutex)
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

func (s *Kubernetes) portForward(ctx context.Context, namespace, serviceName string, ports []string, mutex *sync.Mutex) (chan struct{}, error) {
	service, err := s.clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pods, err := s.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(service.Spec.Selector).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, errors.New("no pods found for service selector")
	}
	podName := pods.Items[0].Name

	roundTripper, upgrader, err := spdy.RoundTripperFor(s.restClientCfg)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP, err := url.Parse(s.restClientCfg.Host)
	if err != nil {
		return nil, err
	}

	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP.Host}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, "POST", &serverURL)

	readyChan := make(chan struct{}, 1)
	stopChan := make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return nil, err
	}

	reconnect(forwarder, readyChan, mutex)

	return stopChan, nil
}

func reconnect(forwarder *portforward.PortForwarder, readyChan chan struct{}, mutex *sync.Mutex) {
	mutex.Lock()
	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			if errors.Is(err, portforward.ErrLostConnectionToPod) {
				reconnect(forwarder, readyChan, mutex)
				return
			}
			panic(err.Error())
		}
	}()
	<-readyChan // This line will block until the port forwarding is ready to get traffic.
	mutex.Unlock()
}

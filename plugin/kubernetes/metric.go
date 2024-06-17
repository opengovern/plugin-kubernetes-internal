package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

func (s *Kubernetes) DiscoverAndPortForwardPrometheusCompatible(ctx context.Context, reconnectMutex *sync.Mutex) (chan struct{}, string, error) {
	var svc *corev1.Service
	var err error
	stopChan := make(chan struct{}, 1)

	svc, err = s.findPrometheusService(ctx)
	if err != nil {
		return nil, "", err
	}
	if svc != nil {
		err = s.portForward(ctx, svc.Namespace, svc.Name, []string{"9090:9090"}, reconnectMutex, stopChan)
		if err != nil {
			return nil, "", err
		}
		return stopChan, "http://localhost:9090", nil
	}

	svc, err = s.findVictoriaMetricsClusterSelectService(ctx)
	if err != nil {
		return nil, "", err
	}
	if svc != nil {
		err = s.portForward(ctx, svc.Namespace, svc.Name, []string{"8481:8481"}, reconnectMutex, stopChan)
		if err != nil {
			return nil, "", err
		}
		return stopChan, "http://localhost:8481/select/0/prometheus", nil
	}

	svc, err = s.findVictoriaMetricsSingleServerService(ctx)
	if err != nil {
		return nil, "", err
	}
	if svc != nil {
		err = s.portForward(ctx, svc.Namespace, svc.Name, []string{"8429:8429"}, reconnectMutex, stopChan)
		if err != nil {
			return nil, "", err
		}
		return stopChan, "http://localhost:8429", nil
	}

	return nil, "", errors.New("no prometheus compatible service found - try passing the prometheus compatible endpoint with the --prom-address flag e.g. --prom-address 'http://localhost:9090'")
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

func (s *Kubernetes) findVictoriaMetricsClusterSelectService(ctx context.Context) (*corev1.Service, error) {
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
			if strings.Contains(svc.Name, "vmselect") {
				for _, port := range svc.Spec.Ports {
					if port.Port == 8481 {
						return &svc, nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (s *Kubernetes) findVictoriaMetricsSingleServerService(ctx context.Context) (*corev1.Service, error) {
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
			if strings.Contains(svc.Name, "vmsingle") {
				for _, port := range svc.Spec.Ports {
					if port.Port == 8429 {
						return &svc, nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (s *Kubernetes) portForward(ctx context.Context, namespace, serviceName string, ports []string, mutex *sync.Mutex, stopChan chan struct{}) error {
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
	fmt.Printf("%s - connecting\n", serverURL.Path)
	s.reconnect(upgrader, ports, roundTripper, serverURL, mutex, stopChan)

	return nil
}

func (s *Kubernetes) reconnect(upgrader spdy.Upgrader, ports []string, roundTripper http.RoundTripper, serverURL url.URL, mutex *sync.Mutex, stopChan chan struct{}) {
	fmt.Printf("%s - reconnect\n", serverURL.Path)
	readyChan := make(chan struct{}, 1)

	mutex.Lock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("%s - %s\n", serverURL.Path, r)
			}

			s.reconnect(upgrader, ports, roundTripper, serverURL, mutex, stopChan)
			return
		}()

		out, errOut := new(bytes.Buffer), new(bytes.Buffer)

		dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, "POST", &serverURL)

		forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
		if err != nil {
			fmt.Printf("%s - %v\n", serverURL.Path, err)
			return
		}

		err = forwarder.ForwardPorts()
		if err != nil {
			fmt.Printf("%s - %v\n", serverURL.Path, err)
			return
		}
	}()
	<-readyChan // This line will block until the port forwarding is ready to get traffic.
	mutex.Unlock()
}

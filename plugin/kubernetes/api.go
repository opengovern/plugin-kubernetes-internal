package kubernetes

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type Kubernetes struct {
	cfg       *restclient.Config
	clientset *kubernetes.Clientset
}

func NewKubernetes(cfg *restclient.Config) (*Kubernetes, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Kubernetes{clientset: clientset}, nil
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

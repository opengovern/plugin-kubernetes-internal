package kubernetes

import (
	"context"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

func GetConfig(ctx context.Context, kubeContextName *string) (*restclient.Config, error) {
	var err error

	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")

	var kubeCfg *restclient.Config
	if kubeContextName == nil || *kubeContextName == "" {
		kubeCfg, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, err
		}
	} else {
		kubeCfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
			&clientcmd.ConfigOverrides{
				CurrentContext: *kubeContextName,
			}).ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	return kubeCfg, nil
}

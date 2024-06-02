package kubernetes

import (
	"context"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

func GetConfig(ctx context.Context, kubeContextName *string) (*restclient.Config, *api.Config, error) {
	var err error

	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	kubeConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return nil, nil, err
	}
	var restclientConfig *restclient.Config
	if kubeContextName == nil || *kubeContextName == "" {
		restclientConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, nil, err
		}
	} else {
		restclientConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
			&clientcmd.ConfigOverrides{
				CurrentContext: *kubeContextName,
			}).ClientConfig()
		if err != nil {
			return nil, nil, err
		}
		kubeConfig.CurrentContext = *kubeContextName
	}

	return restclientConfig, kubeConfig, nil
}

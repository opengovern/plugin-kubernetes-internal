package kubernetes

import (
	"context"
	"errors"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

func GetConfig(ctx context.Context, kubeContextName *string) (*restclient.Config, *api.Config, error) {
	var err error
	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")

	if _, err = os.Stat(kubeConfigPath); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	} else if os.IsNotExist(err) { // check if running in cluster
		config, err := restclient.InClusterConfig()
		if err == nil {
			return config, nil, nil
		}
	} else if err == nil {
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

	return nil, nil, errors.New("unable to get kubernetes config")
}

package prometheus

import (
	"context"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"sync"
	"time"
)

type Config struct {
	Address string `json:"address"`

	reconnectWait sync.Mutex
}

func GetConfig(address *string, agentDisabled bool, client *kaytuKubernetes.Kubernetes) (*Config, error) {
	cfg := Config{
		reconnectWait: sync.Mutex{},
	}

	if address != nil {
		cfg.Address = *address
	}

	if cfg.Address == "" && !agentDisabled {
		_, err := client.DiscoverAndPortForwardKaytuAgent(context.Background(), &cfg.reconnectWait)
		if err != nil {
			return nil, err
		}
		time.Sleep(1 * time.Second)
		cfg.Address = "localhost:8001"
	}

	return &cfg, nil
}

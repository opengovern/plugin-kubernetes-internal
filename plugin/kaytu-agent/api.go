package prometheus

import (
	"context"
	kaytuAgent "github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type KaytuAgent struct {
	cfg        *Config
	discovered bool
	client     kaytuAgent.AgentClient
}

func NewKaytuAgent(cfg *Config) (*KaytuAgent, error) {
	agent := KaytuAgent{
		cfg:        cfg,
		discovered: cfg.Address != "",
	}
	if agent.discovered {
		conn, err := grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		agent.client = kaytuAgent.NewAgentClient(conn)
	}

	return &agent, nil
}

func (a KaytuAgent) Ping(ctx context.Context) error {
	_, err := a.client.Ping(context.Background(), &kaytuAgent.PingMessage{})
	return err
}

func (a KaytuAgent) DownloadReport(cmd string) ([]byte, error) {
	resp, err := a.client.GetReport(context.Background(), &kaytuAgent.GetReportRequest{
		Command: cmd,
	})
	if err != nil {
		return nil, err
	}
	return resp.Report, nil
}

func (a KaytuAgent) IsDiscovered() bool {
	return a.discovered
}

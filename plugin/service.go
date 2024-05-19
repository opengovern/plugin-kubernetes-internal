package plugin

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes/plugin/preferences"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
)

type KubernetesPlugin struct {
	cfg    aws.Config
	stream golang.Plugin_RegisterClient
	//processor processor2.Processor
}

func NewPlugin() *KubernetesPlugin {
	return &KubernetesPlugin{}
}

func (p *KubernetesPlugin) GetConfig() golang.RegisterConfig {
	return golang.RegisterConfig{
		Name:     "kaytu-io/plugin-kubernetes",
		Version:  version.VERSION,
		Provider: "aws",
		Commands: []*golang.Command{
			{
				Name:        "ec2-instance",
				Description: "Optimize your AWS EC2 Instances",
				Flags: []*golang.Flag{
					{
						Name:        "profile",
						Default:     "",
						Description: "AWS profile for authentication",
						Required:    false,
					},
				},
				DefaultPreferences: preferences.DefaultEC2Preferences,
			},
		},
	}
}

func (p *KubernetesPlugin) SetStream(stream golang.Plugin_RegisterClient) {
	p.stream = stream
}

func (p *KubernetesPlugin) StartProcess(command string, flags map[string]string, kaytuAccessToken string, jobQueue *sdk.JobQueue) error {

	return nil
}

func (p *KubernetesPlugin) ReEvaluate(evaluate *golang.ReEvaluate) {
	//p.processor.ReEvaluate(evaluate.Id, evaluate.Preferences)
}

package preferences

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var DefaultPodsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesPod", Key: "CPUBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesPod", Key: "MemoryBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
}

var DefaultDeploymentsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesDeployment", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
}

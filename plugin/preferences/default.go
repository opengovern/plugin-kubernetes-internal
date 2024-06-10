package preferences

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var DefaultPodsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesPod", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesPod", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesPod", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesPod", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
}

var DefaultDeploymentsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesDeployment", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
}

var DefaultStatefulsetsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesStatefulset", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
}

var DefaultDaemonsetsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesDaemonsets", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
}

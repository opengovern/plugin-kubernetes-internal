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
	{Service: "KubernetesPod", Key: "MinCpuRequest", IsNumber: true, Value: wrapperspb.String("0.1"), PreventPinning: true, Unit: "milli cores"},
	{Service: "KubernetesPod", Key: "MinMemoryRequest", IsNumber: true, Value: wrapperspb.String("100"), PreventPinning: true, Unit: "MB"},
}

var DefaultDeploymentsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesDeployment", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDeployment", Key: "MinCpuRequest", IsNumber: true, Value: wrapperspb.String("0.1"), PreventPinning: true, Unit: "milli cores"},
	{Service: "KubernetesDeployment", Key: "MinMemoryRequest", IsNumber: true, Value: wrapperspb.String("100"), PreventPinning: true, Unit: "MB"},
}

var DefaultStatefulsetsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesStatefulset", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesStatefulset", Key: "MinCpuRequest", IsNumber: true, Value: wrapperspb.String("0.1"), PreventPinning: true, Unit: "milli cores"},
	{Service: "KubernetesStatefulset", Key: "MinMemoryRequest", IsNumber: true, Value: wrapperspb.String("100"), PreventPinning: true, Unit: "MB"},
}

var DefaultDaemonsetsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesDaemonsets", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesDaemonsets", Key: "MinCpuRequest", IsNumber: true, Value: wrapperspb.String("0.1"), PreventPinning: true, Unit: "milli cores"},
	{Service: "KubernetesDaemonsets", Key: "MinMemoryRequest", IsNumber: true, Value: wrapperspb.String("100"), PreventPinning: true, Unit: "MB"},
}

var DefaultJobsPreferences = []*golang.PreferenceItem{
	{Service: "KubernetesJobs", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesJobs", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesJobs", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesJobs", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "KubernetesJobs", Key: "MinCpuRequest", IsNumber: true, Value: wrapperspb.String("0.1"), PreventPinning: true, Unit: "milli cores"},
	{Service: "KubernetesJobs", Key: "MinMemoryRequest", IsNumber: true, Value: wrapperspb.String("100"), PreventPinning: true, Unit: "MB"},
}

var DefaultAllPreferences = append(DefaultPodsPreferences,
	append(DefaultDeploymentsPreferences,
		append(DefaultStatefulsetsPreferences,
			append(DefaultDaemonsetsPreferences,
				DefaultJobsPreferences...,
			)...,
		)...,
	)...,
)

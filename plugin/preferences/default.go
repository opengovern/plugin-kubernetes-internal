package preferences

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var DefaultKubernetesPreferences = []*golang.PreferenceItem{
	{Service: "Kubernetes", Key: "CPURequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "Kubernetes", Key: "MemoryRequestBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "Kubernetes", Key: "CPULimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "Kubernetes", Key: "MemoryLimitBreathingRoom", IsNumber: true, Value: wrapperspb.String("10"), PreventPinning: true, Unit: "%"},
	{Service: "Kubernetes", Key: "MinCpuRequest", IsNumber: true, Value: wrapperspb.String("0.1"), PreventPinning: true, Unit: "milli cores"},
	{Service: "Kubernetes", Key: "MinMemoryRequest", IsNumber: true, Value: wrapperspb.String("100"), PreventPinning: true, Unit: "MB"},
}

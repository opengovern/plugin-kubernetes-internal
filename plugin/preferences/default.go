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
	{Service: "Kubernetes", Key: "LeaveCPULimitEmpty", Value: wrapperspb.String("false"), PossibleValues: []string{"false", "true"}, PreventPinning: true},
	{Service: "Kubernetes", Key: "EqualMemoryRequestLimit", Value: wrapperspb.String("false"), PossibleValues: []string{"false", "true"}, PreventPinning: true},
	{Service: "Kubernetes", Key: "NodeCPUBreathingRoom", IsNumber: true, Value: wrapperspb.String("15"), PreventPinning: true},
	{Service: "Kubernetes", Key: "NodeMemoryBreathingRoom", IsNumber: true, Value: wrapperspb.String("15"), PreventPinning: true},
	{Service: "Kubernetes", Key: "NodePodCountBreathingRoom", IsNumber: true, Value: wrapperspb.String("5"), PreventPinning: true},
}

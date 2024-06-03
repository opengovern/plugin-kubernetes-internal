package shared

import corev1 "k8s.io/api/core/v1"

func GetContainerRequestLimits(container corev1.Container) (cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64) {
	if container.Resources.Requests.Cpu() != nil {
		v := container.Resources.Requests.Cpu().AsApproximateFloat64()
		if v != 0 {
			cpuRequest = &v
		}
	}
	if container.Resources.Limits.Cpu() != nil {
		v := container.Resources.Limits.Cpu().AsApproximateFloat64()
		if v != 0 {
			cpuLimit = &v
		}
	}
	if container.Resources.Requests.Memory() != nil {
		v := container.Resources.Requests.Memory().AsApproximateFloat64()
		if v != 0 {
			memoryRequest = &v
		}
	}
	if container.Resources.Limits.Memory() != nil {
		v := container.Resources.Limits.Memory().AsApproximateFloat64()
		if v != 0 {
			memoryLimit = &v
		}
	}
	return cpuRequest, cpuLimit, memoryRequest, memoryLimit
}

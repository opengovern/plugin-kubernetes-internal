package shared

import (
	"fmt"
	"strings"
)

func CpuConfiguration(request, limit *float64) string {
	var strs []string
	if request != nil {
		strs = append(strs, fmt.Sprintf("Request: %.2f Cores", *request))
	}

	if limit != nil {
		strs = append(strs, fmt.Sprintf("Limit: %.2f Cores", *limit))
	}

	return strings.Join(strs, ", ")
}
func MemoryConfiguration(request, limit *float64) string {
	var strs []string
	if request != nil {
		strs = append(strs, fmt.Sprintf("Request: %.2f GB", *request/(1024*1024*1024)))
	}

	if limit != nil {
		strs = append(strs, fmt.Sprintf("Limit: %.2f GB", *limit/(1024*1024*1024)))
	}

	return strings.Join(strs, ", ")
}

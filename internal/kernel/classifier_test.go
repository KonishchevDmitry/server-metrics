package kernel

import (
	"context"
	"testing"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestClassify(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zap.NewNop().Sugar())

	tests := []struct {
		name     string
		messages []string
		result   []errorType
	}{{
		name: "AMD IOMMU errors",
		messages: []string{
			"kfd kfd: amdgpu: Failed to resume IOMMU for device 1002:9874",
			"kfd kfd: amdgpu: device 1002:9874 NOT added due to errors",
		},
	}, {
		name: "Mixed",
		messages: []string{
			"kfd kfd: amdgpu: Failed to resume IOMMU for device 1002:9874",
			"kfd kfd: amdgpu: device 1002:9874 NOT added due to errors",
			"Some unknown error",
		},
		result: []errorType{errorTypeUnknown},
	}}

	for _, c := range tests {
		c := c
		t.Run(c.name, func(t *testing.T) {
			result := classify(ctx, c.messages)
			require.Equal(t, c.result, result)
		})
	}
}

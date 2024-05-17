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
		name: "UBSAN errors",
		messages: []string{
			"UBSAN: array-index-out-of-bounds in /build/linux-yrLejD/linux-6.8.0/drivers/gpu/drm/amd/amdgpu/../pm/powerplay/hwmgr/processpptables.c:1249:61",
			"index 1 is out of range for type 'ATOM_PPLIB_VCE_Clock_Voltage_Limit_Record [1]'",
		},
	}, {
		name: "NMI received for unknown reason",
		messages: []string{
			"Uhhuh. NMI received for unknown reason 2d on CPU 0.",
			"Dazed and confused, but trying to continue",
		},
		result: []errorType{errorTypeUnexpectedNMI},
	}}

	for _, c := range tests {
		c := c
		t.Run(c.name, func(t *testing.T) {
			result := classify(ctx, c.messages)
			require.Equal(t, c.result, result)
		})
	}

	t.Run("mixed", func(t *testing.T) {
		var messages []string

		for _, c := range tests {
			messages = append(messages, c.messages...)
		}
		messages = append(messages, "Some unknown error")

		result := classify(ctx, messages)
		require.Equal(t, []errorType{errorTypeUnexpectedNMI, errorTypeUnknown}, result)
	})
}

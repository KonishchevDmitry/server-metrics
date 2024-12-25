package kernel

import (
	"context"
	"strings"
	"testing"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestClassify(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zap.NewNop().Sugar())

	tests := []struct {
		name     string
		messages string
		result   []errorType
	}{{
		name: "AMD IOMMU errors",
		messages: heredoc.Doc(`
			kfd kfd: amdgpu: Failed to resume IOMMU for device 1002:9874
			kfd kfd: amdgpu: device 1002:9874 NOT added due to errors
		`),
	}, {
		name:     "Missing hardware watchdog",
		messages: `sp5100-tco sp5100-tco: Watchdog hardware is disabled`,
	}, {
		name: "Hotplug initialization",
		messages: heredoc.Doc(`
			shpchp 0000:01:00.0: pci_hp_register failed with error -16
			shpchp 0000:01:00.0: Slot initialization failed
		`),
	}, {
		name: "UBSAN errors",
		messages: heredoc.Doc(`
			UBSAN: array-index-out-of-bounds in /build/linux-yrLejD/linux-6.8.0/drivers/gpu/drm/amd/amdgpu/../pm/powerplay/hwmgr/processpptables.c:1249:61
			index 1 is out of range for type 'ATOM_PPLIB_VCE_Clock_Voltage_Limit_Record [1]'
		`),
	}, {
		name: "NMI received for unknown reason",
		messages: heredoc.Doc(`
			Uhhuh. NMI received for unknown reason 2d on CPU 0.
			Dazed and confused, but trying to continue
		`),
		result: []errorType{errorTypeUnexpectedNMI},
	}}

	classifier := newClassifier(true)

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			messages := strings.Split(strings.TrimRight(c.messages, "\n"), "\n")
			result := classifier.classify(ctx, messages)
			require.Equal(t, c.result, result)
		})
	}

	t.Run("mixed", func(t *testing.T) {
		var mixedMessages []string

		for _, c := range tests {
			messages := strings.Split(strings.TrimRight(c.messages, "\n"), "\n")
			mixedMessages = append(mixedMessages, messages...)
		}
		mixedMessages = append(mixedMessages, "Some unknown error")

		result := classifier.classify(ctx, mixedMessages)
		require.Equal(t, []errorType{errorTypeUnexpectedNMI, errorTypeUnknown}, result)
	})
}

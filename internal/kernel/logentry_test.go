package kernel

import (
	"log/syslog"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseLogEntry(t *testing.T) {
	tests := []struct {
		name string
		data string

		facility syslog.Priority
		severity syslog.Priority
		time     time.Duration
		message  string
	}{
		{
			name: "simple",
			data: "4,364,481851,-;[Firmware Bug]: HEST: Table contents overflow for hardware error source: 2.\n",

			facility: syslog.LOG_KERN,
			severity: syslog.LOG_WARNING,
			time:     481851 * time.Microsecond,
			message:  "[Firmware Bug]: HEST: Table contents overflow for hardware error source: 2.",
		},
		{
			name: "non-kernel",
			data: "30,975,21641518,-;systemd[1]: Starting Set the console keyboard layout...\n",

			facility: syslog.LOG_DAEMON,
			severity: syslog.LOG_INFO,
			time:     21641518 * time.Microsecond,
			message:  "systemd[1]: Starting Set the console keyboard layout...",
		},
		{
			name: "complex",
			data: heredoc.Doc(`
				6,861,8812129,-;amdgpu 0000:00:01.0: amdgpu: Fetched VBIOS from VFCT
				SUBSYSTEM=pci
				DEVICE=+pci:0000:00:01.0
			`),

			facility: syslog.LOG_KERN,
			severity: syslog.LOG_INFO,
			time:     8812129 * time.Microsecond,
			message:  "amdgpu 0000:00:01.0: amdgpu: Fetched VBIOS from VFCT",
		},
	}

	for _, c := range tests {
		c := c
		t.Run(c.name, func(t *testing.T) {
			entry, ok := parseLogEntry([]byte(c.data))
			require.True(t, ok)
			require.Equal(t, c.facility, entry.facility())
			require.Equal(t, c.severity, entry.severity())
			require.Equal(t, c.time, entry.time)
			require.Equal(t, c.message, entry.message)
		})
	}
}

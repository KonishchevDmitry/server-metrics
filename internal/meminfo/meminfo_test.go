package meminfo

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseMeminfo(t *testing.T) {
	meminfo, err := parseMeminfo(strings.NewReader(heredoc.Doc(`
		MemTotal:        7562404 kB
		MemFree:         3136456 kB
		MemAvailable:    5850912 kB
		Buffers:          237736 kB
		Cached:          1340968 kB
		SwapCached:        42784 kB
		Active:           826096 kB
		Inactive:        1422040 kB
		Active(anon):     447320 kB
		Inactive(anon):   221404 kB
		Active(file):     378776 kB
		Inactive(file):  1200636 kB
		Unevictable:          24 kB
		Mlocked:               0 kB
		SwapTotal:       4188156 kB
		SwapFree:        3609480 kB
		Zswap:             75708 kB
		Zswapped:         258744 kB
		Dirty:              1488 kB
		Writeback:             0 kB
		AnonPages:        658628 kB
		Mapped:           710580 kB
		Shmem:               356 kB
		KReclaimable:    1436320 kB
		Slab:            1918820 kB
		SReclaimable:    1436320 kB
		SUnreclaim:       482500 kB
		KernelStack:       11776 kB
		PageTables:        30208 kB
		SecPageTables:         0 kB
		NFS_Unstable:          0 kB
		Bounce:                0 kB
		WritebackTmp:          0 kB
		CommitLimit:     7969356 kB
		Committed_AS:   10819956 kB
		VmallocTotal:   34359738367 kB
		VmallocUsed:       59652 kB
		VmallocChunk:          0 kB
		Percpu:             3552 kB
		HardwareCorrupted:     0 kB
		AnonHugePages:      4096 kB
		ShmemHugePages:        0 kB
		ShmemPmdMapped:        0 kB
		FileHugePages:         0 kB
		FilePmdMapped:         0 kB
		CmaTotal:              0 kB
		CmaFree:               0 kB
		Unaccepted:            0 kB
		HugePages_Total:       0
		HugePages_Free:        0
		HugePages_Rsvd:        0
		HugePages_Surp:        0
		Hugepagesize:       2048 kB
		Hugetlb:               0 kB
		DirectMap4k:      273412 kB
		DirectMap2M:     5451776 kB
		DirectMap1G:     2097152 kB
	`)))
	require.NoError(t, err)
	require.Len(t, meminfo, 57)

	require.Equal(t, int64(4188156*1024), meminfo["SwapTotal"])
	require.Equal(t, int64(0), meminfo["HugePages_Total"])
}

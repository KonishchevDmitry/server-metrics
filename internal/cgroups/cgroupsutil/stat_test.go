package cgroupsutil

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseStat(t *testing.T) {
	stat, err := parseStat(strings.NewReader(heredoc.Doc(`
		anon 780599296
		file 6221778944
		kernel_stack 6471680
		pagetables 11464704
		percpu 2328576
		sock 3518464
		shmem 3112960
		file_mapped 278974464
		file_dirty 2195456
		file_writeback 0
		swapcached 6365184
		anon_thp 4194304
		file_thp 0
		shmem_thp 0
		inactive_anon 633528320
		active_anon 148041728
		inactive_file 2885767168
		active_file 3324174336
		unevictable 19595264
		slab_reclaimable 326309080
		slab_unreclaimable 11402288
		slab 337711368
		workingset_refault_anon 2195
		workingset_refault_file 872129
		workingset_activate_anon 524
		workingset_activate_file 354134
		workingset_restore_anon 49
		workingset_restore_file 139163
		workingset_nodereclaim 108800
		pgfault 87121489
		pgmajfault 28073
		pgrefill 2180219
		pgscan 13213710
		pgsteal 12706506
		pgactivate 2356269
		pgdeactivate 2089123
		pglazyfree 129565
		pglazyfreed 2571
		thp_fault_alloc 314
		thp_collapse_alloc 89
	`)))
	require.NoError(t, err)
	require.Len(t, stat, 40)
	require.Equal(t, int64(337711368), stat["slab"])
}

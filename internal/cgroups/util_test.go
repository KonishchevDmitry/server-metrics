package cgroups

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseStat(t *testing.T) {
	stat, exists, err := parseStat(strings.NewReader(heredoc.Doc(`
		cache 1757184
		rss 5406720
		rss_huge 0
		shmem 0
		mapped_file 1216512
		dirty 0
		writeback 0
		pgpgin 1782
		pgpgout 0
		pgfault 1716
		pgmajfault 0
		inactive_anon 0
		active_anon 3919872
		inactive_file 3244032
		active_file 135168
		unevictable 0
		hierarchical_memory_limit 9223372036854771712
		total_cache 1757184
		total_rss 5406720
		total_rss_huge 0
		total_shmem 0
		total_mapped_file 1216512
		total_dirty 0
		total_writeback 0
		total_pgpgin 1782
		total_pgpgout 0
		total_pgfault 1716
		total_pgmajfault 0
		total_inactive_anon 0
		total_active_anon 3919872
		total_inactive_file 3244032
		total_active_file 135168
		total_unevictable 0
	`)))
	require.NoError(t, err)
	require.True(t, exists)
	require.Len(t, stat, 33)
	require.Equal(t, int64(1757184), stat["total_cache"])
}

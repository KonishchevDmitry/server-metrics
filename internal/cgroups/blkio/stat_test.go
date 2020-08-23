package blkio

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseStat(t *testing.T) {
	stats, ok, err := parseStat(strings.NewReader(heredoc.Doc(`
		8:16 Read 214
		8:16 Write 71
		8:16 Sync 284
		8:16 Async 1
		8:16 Discard 0
		8:16 Total 285
		8:0 Read 395
		8:0 Write 71
		8:0 Sync 465
		8:0 Async 1
		8:0 Discard 0
		8:0 Total 466
		9:0 Read 609
		9:0 Write 55
		9:0 Sync 663
		9:0 Async 1
		9:0 Discard 0
		9:0 Total 664
		Total 1415
	`)))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []stat{
		{device: "8:16", read: 214, write: 71},
		{device: "8:0", read: 395, write: 71},
		{device: "9:0", read: 609, write: 55},
	}, stats)
}

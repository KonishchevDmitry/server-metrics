package cgroupsutil

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseNamedStat(t *testing.T) {
	stats, err := parseNamedStat("io.stat", strings.NewReader(heredoc.Doc(`
		9:0 rbytes=6833214464 wbytes=12408209408 rios=219345 wios=461266 dbytes=0 dios=0
		8:16 rbytes=2504615424 wbytes=12477784576 rios=71571 wios=232169 dbytes=0 dios=0
		8:0 rbytes=4348810240 wbytes=12477784064 rios=103424 wios=232614 dbytes=0 dios=0
		7:7 7:6 7:5 7:4 7:3 7:2 rbytes=14336 wbytes=0 rios=11 wios=0 dbytes=0 dios=0
		7:1 rbytes=2741248 wbytes=0 rios=181 wios=0 dbytes=0 dios=0
		7:0 rbytes=1093632 wbytes=0 rios=53 wios=0 dbytes=0 dios=0
	`)))
	require.NoError(t, err)
	require.Len(t, stats, 11)

	require.Equal(t, int64(12477784064), stats["8:0"].stat["wbytes"])
	require.Equal(t, int64(14336), stats["7:6"].stat["rbytes"])
	require.Equal(t, int64(14336), stats["7:3"].stat["rbytes"])
}

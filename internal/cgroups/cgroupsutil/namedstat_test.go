package cgroupsutil

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseNamedStat(t *testing.T) {
	stats, err := parseNamedStat("io.stat", strings.NewReader(heredoc.Doc(`
		8:16 rbytes=1001971712 wbytes=25937455104 rios=53916 wios=1160495 dbytes=0 dios=0
		8:0 rbytes=2096605696 wbytes=25937455616 rios=118689 wios=1162156 dbytes=0 dios=0
		7:6 rbytes=14336 wbytes=0 rios=11 wios=0 dbytes=0 dios=0
		7:5 rbytes=1093632 wbytes=0 rios=52 wios=0 dbytes=0 dios=0
		7:4 rbytes=2756608 wbytes=0 rios=178 wios=0 dbytes=0 dios=0
		7:3 rbytes=367616 wbytes=0 rios=48 wios=0 dbytes=0 dios=0
		7:2 rbytes=353280 wbytes=0 rios=41 wios=0 dbytes=0 dios=0
		7:1 rbytes=1093632 wbytes=0 rios=53 wios=0 dbytes=0 dios=0
		7:0 rbytes=1095680 wbytes=0 rios=62 wios=0 dbytes=0 dios=0
	`)))
	require.NoError(t, err)
	require.Len(t, stats, 9)
	require.Equal(t, int64(25937455616), stats["8:0"].stat["wbytes"])
}

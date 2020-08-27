package cpu

import (
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/require"
)

func TestParseStat(t *testing.T) {
	usage, exists, err := parseStat(strings.NewReader(heredoc.Doc(`
		cpu user system
		0 30835357791376 2754524018447
		1 25206823626112 2308261173051
		2 30104247421960 2868375441021
		3 24619276914717 2375300705372
	`)))
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, stat{
		user:   30835357791376 + 25206823626112 + 30104247421960 + 24619276914717,
		system: 2754524018447 + 2308261173051 + 2868375441021 + 2375300705372,
	}, usage)
}

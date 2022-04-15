package bytes

import (
	"BFT/libs/rand"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestTransfer(t *testing.T) {
	bz := rand.Bytes(32)
	hbz := HexBytes(bz)
	t.Log(hbz.String())
	res := hbz.String()
	t.Log(strings.ToLower(res))
	hbz2, err := FromString(strings.ToLower(res))
	require.NoError(t, err)
	require.Equal(t, hbz, hbz2)
}

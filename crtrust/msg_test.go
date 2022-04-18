package crtrust

import (
	"fmt"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	"testing"
)

func test(b map[string]struct{}) {
	b["a"] = struct{}{}
}

func TestSlice(t *testing.T) {
	var m []protoabci.ValidatorUpdate
	fmt.Println(m == nil, len(m) == 0)
}

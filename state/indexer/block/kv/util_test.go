package kv

import (
	"fmt"
	"github.com/google/orderedcode"
	"testing"
)

func TestOrder(t *testing.T) {
	var height int64 = 871231232131
	var h int64
	var s string
	v, _ := orderedcode.Append(nil, "block.height", height)
	r, _ := orderedcode.Parse(string(v), &s, &h)
	fmt.Println(v, s, h, r)
}

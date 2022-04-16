package rpc

import (
	httprpc "github.com/232425wxy/BFT/rpc/client/http"
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRpc(t *testing.T) {
	client, err := httprpc.New("localhost:36657", "/websocket")
	require.NoError(t, err)
	res, err := client.ABCIQuery(context.Background(), "87C484A50EC2A1E8E9150F457E1EF609BF00F92F291C6C0633F186DC85A2EA7")
	require.NoError(t, err)
	fmt.Println(res)
}

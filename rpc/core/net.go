package core

import (
	"errors"
	"strings"

	"BFT/gossip"
	coretypes "BFT/rpc/core/types"
	rpctypes "BFT/rpc/jsonrpc/types"
)

// NetInfo returns network info.
func NetInfo(ctx *rpctypes.Context) (*coretypes.ResultNetInfo, error) {
	peersList := env.P2PPeers.Peers().List()
	peers := make([]coretypes.Peer, 0, len(peersList))
	for _, peer := range peersList {
		nodeInfo := peer.NodeInfo()
		peers = append(peers, coretypes.Peer{
			NodeInfo:         nodeInfo,
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Status(),
			RemoteIP:         peer.RemoteIP().String(),
		})
	}
	return &coretypes.ResultNetInfo{
		Listening: env.P2PTransport.IsListening(),
		Listeners: env.P2PTransport.Listeners(),
		NPeers:    len(peers),
		Peers:     peers,
	}, nil
}

// UnsafeDialSeeds dials the given seeds (comma-separated id@IP:PORT).
func UnsafeDialSeeds(ctx *rpctypes.Context, seeds []string) (*coretypes.ResultDialSeeds, error) {
	if len(seeds) == 0 {
		return &coretypes.ResultDialSeeds{}, errors.New("no seeds provided")
	}
	env.Logger.Infow("DialSeeds", "seeds", seeds)
	if err := env.P2PPeers.DialPeersAsync(seeds); err != nil {
		return &coretypes.ResultDialSeeds{}, err
	}
	return &coretypes.ResultDialSeeds{Log: "Dialing seeds in progress. See /net_info for details"}, nil
}

// UnsafeDialPeers dials the given peers (comma-separated id@IP:PORT),
// optionally making them persistent.
func UnsafeDialPeers(ctx *rpctypes.Context, peers []string, persistent, private bool) (
	*coretypes.ResultDialPeers, error) {
	if len(peers) == 0 {
		return &coretypes.ResultDialPeers{}, errors.New("no peers provided")
	}

	ids, err := getIDs(peers)
	if err != nil {
		return &coretypes.ResultDialPeers{}, err
	}

	env.Logger.Infow("DialPeers", "peers", peers, "persistent", persistent, "private", private)

	if persistent {
		if err := env.P2PPeers.AddPersistentPeers(peers); err != nil {
			return &coretypes.ResultDialPeers{}, err
		}
	}

	if private {
		if err := env.P2PPeers.AddPrivatePeerIDs(ids); err != nil {
			return &coretypes.ResultDialPeers{}, err
		}
	}

	if err := env.P2PPeers.DialPeersAsync(peers); err != nil {
		return &coretypes.ResultDialPeers{}, err
	}

	return &coretypes.ResultDialPeers{Log: "Dialing peers in progress. See /net_info for details"}, nil
}

// Genesis returns genesis file.
func Genesis(ctx *rpctypes.Context) (*coretypes.ResultGenesis, error) {
	return &coretypes.ResultGenesis{Genesis: env.GenDoc}, nil
}

func getIDs(peers []string) ([]string, error) {
	ids := make([]string, 0, len(peers))

	for _, peer := range peers {

		spl := strings.Split(peer, "@")
		if len(spl) != 2 {
			return nil, gossip.ErrNetAddressNoID{Addr: peer}
		}
		ids = append(ids, spl[0])

	}
	return ids, nil
}

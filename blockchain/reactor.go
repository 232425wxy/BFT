package blockchain

import (
	"BFT/gossip"
	"BFT/libs/log"
	protoblockchain "BFT/proto/blockchain"
	sm "BFT/state"
	"BFT/store"
	"BFT/types"
	"fmt"
	"reflect"
	"time"
)

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)

	trySyncIntervalMS = 10

	// stop syncing when last block's time is
	// within this much of the system time.
	// stopSyncingDurationMinutes = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 1
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(state sm.State, skipWAL bool)
}

type peerError struct {
	err    error
	peerID gossip.ID
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	gossip.BaseReactor

	// immutable
	initialState sm.State

	blockExec *sm.BlockExecutor
	store     *store.BlockStore
	pool      *BlockPool
	fastSync  bool

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *store.BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	requestsCh := make(chan BlockRequest, maxTotalRequesters)

	const capacity = 1000                      // must be bigger than peers count
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	startHeight := store.Height() + 1
	if startHeight == 1 {
		startHeight = state.InitialHeight
	}
	pool := NewBlockPool(startHeight, requestsCh, errorsCh)

	bcR := &BlockchainReactor{
		initialState: state,
		blockExec:    blockExec,
		store:        store,
		pool:         pool,
		fastSync:     fastSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
	}
	bcR.BaseReactor = *gossip.NewBaseReactor("BlockchainReactor", bcR)
	return bcR
}

// SetLogger implements service.Service by setting the logger on reactor and pool.
func (bcR *BlockchainReactor) SetLogger(l log.CRLogger) {
	bcR.BaseService.Logger = l
	bcR.pool.Logger = l
}

// OnStart implements service.Service.
func (bcR *BlockchainReactor) OnStart() error {
	if bcR.fastSync {
		err := bcR.pool.Start()
		if err != nil {
			return err
		}
		go bcR.poolRoutine(false)
	}
	return nil
}

// SwitchToFastSync is called by the state sync reactor when switching to fast sync.
func (bcR *BlockchainReactor) SwitchToFastSync(state sm.State) error {
	bcR.fastSync = true
	bcR.initialState = state

	bcR.pool.height = state.LastBlockHeight + 1
	err := bcR.pool.Start()
	if err != nil {
		return err
	}
	go bcR.poolRoutine(true)
	return nil
}

// OnStop implements service.Service.
func (bcR *BlockchainReactor) OnStop() {
	if bcR.fastSync {
		if err := bcR.pool.Stop(); err != nil {
			bcR.Logger.Errorw("Error stopping pool", "err", err)
		}
	}
}

// GetChannels implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*gossip.ChannelDescriptor {
	return []*gossip.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            5,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: MaxMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcR *BlockchainReactor) AddPeer(peer *gossip.Peer) {
	msgBytes, err := EncodeMsg(&protoblockchain.StatusResponse{
		Base:   bcR.store.Base(),
		Height: bcR.store.Height()})
	if err != nil {
		bcR.Logger.Errorw("could not convert msg to protobuf", "err", err)
		return
	}

	peer.Send(BlockchainChannel, msgBytes)
	// it's OK if send fails. will try later in poolRoutine

	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.SetPeerRange
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer *gossip.Peer, reason interface{}) {
	bcR.pool.RemovePeer(peer.ID())
}

// respondToPeer loads a block and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
func (bcR *BlockchainReactor) respondToPeer(msg *protoblockchain.BlockRequest, src *gossip.Peer) (queued bool) {

	block := bcR.store.LoadBlock(msg.Height)
	if block != nil {
		bl, err := block.ToProto()
		if err != nil {
			bcR.Logger.Errorw("could not convert msg to protobuf", "err", err)
			return false
		}

		msgBytes, err := EncodeMsg(&protoblockchain.BlockResponse{Block: bl})
		if err != nil {
			bcR.Logger.Errorw("could not marshal msg", "err", err)
			return false
		}

		return src.TrySend(BlockchainChannel, msgBytes)
	}

	bcR.Logger.Infow("Peer asking for a block we don't have", "src", src, "height", msg.Height)

	msgBytes, err := EncodeMsg(&protoblockchain.NoBlockResponse{Height: msg.Height})
	if err != nil {
		bcR.Logger.Errorw("could not convert msg to protobuf", "err", err)
		return false
	}

	return src.TrySend(BlockchainChannel, msgBytes)
}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcR *BlockchainReactor) Receive(chID byte, src *gossip.Peer, msgBytes []byte) {
	msg, err := DecodeMsg(msgBytes)
	if err != nil {
		bcR.Logger.Errorw("Error decoding message", "src", src, "chId", chID, "err", err)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	if err = ValidateMsg(msg); err != nil {
		bcR.Logger.Errorw("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	bcR.Logger.Debugw("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *protoblockchain.BlockRequest:
		bcR.respondToPeer(msg, src)
	case *protoblockchain.BlockResponse:
		bi, err := types.BlockFromProto(msg.Block)
		if err != nil {
			bcR.Logger.Errorw("Block content is invalid", "err", err)
			return
		}
		bcR.pool.AddBlock(src.ID(), bi, len(msgBytes))
	case *protoblockchain.StatusRequest:
		// Send peer our state.
		msgBytes, err := EncodeMsg(&protoblockchain.StatusResponse{
			Height: bcR.store.Height(),
			Base:   bcR.store.Base(),
		})
		if err != nil {
			bcR.Logger.Errorw("could not convert msg to protobut", "err", err)
			return
		}
		src.TrySend(BlockchainChannel, msgBytes)
	case *protoblockchain.StatusResponse:
		// Got a peer status. Unverified.
		bcR.pool.SetPeerRange(src.ID(), msg.Base, msg.Height)
	case *protoblockchain.NoBlockResponse:
		bcR.Logger.Debugw("Peer does not have requested block", "peer", src, "height", msg.Height)
	default:
		bcR.Logger.Errorw(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (bcR *BlockchainReactor) poolRoutine(stateSynced bool) {

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	defer trySyncTicker.Stop()

	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	defer statusUpdateTicker.Stop()

	switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)
	defer switchToConsensusTicker.Stop()

	blocksSynced := uint64(0)

	chainID := bcR.initialState.ChainID
	state := bcR.initialState

	lastHundred := time.Now()
	lastRate := 0.0

	didProcessCh := make(chan struct{}, 1)

	go func() {
		for {
			select {
			case <-bcR.Quit():
				return
			case <-bcR.pool.Quit():
				return
			case request := <-bcR.requestsCh:
				peer := bcR.Switch.Peers().Get(request.PeerID)
				if peer == nil {
					continue
				}
				msgBytes, err := EncodeMsg(&protoblockchain.BlockRequest{Height: request.Height})
				if err != nil {
					bcR.Logger.Errorw("could not convert msg to proto", "err", err)
					continue
				}

				queued := peer.TrySend(BlockchainChannel, msgBytes)
				if !queued {
					bcR.Logger.Debugw("Send queue is full, drop block request", "peer", peer.ID(), "height", request.Height)
				}
			case err := <-bcR.errorsCh:
				peer := bcR.Switch.Peers().Get(err.peerID)
				if peer != nil {
					bcR.Switch.StopPeerForError(peer, err)
				}

			case <-statusUpdateTicker.C:
				// ask for status updates
				go bcR.BroadcastStatusRequest() // nolint: errcheck

			}
		}
	}()

FOR_LOOP:
	for {
		select {
		case <-switchToConsensusTicker.C:
			height, numPending, lenRequesters := bcR.pool.GetStatus()
			outbound, inbound, _ := bcR.Switch.NumPeers()
			bcR.Logger.Debugw("Consensus ticker", "numPending", numPending, "total", lenRequesters,
				"outbound", outbound, "inbound", inbound)
			if bcR.pool.IsCaughtUp() {
				bcR.Logger.Infow("Time to switch to consensus reactor!", "height", height)
				if err := bcR.pool.Stop(); err != nil {
					bcR.Logger.Errorw("Error stopping pool", "err", err)
				}
				conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
				if ok {
					conR.SwitchToConsensus(state, blocksSynced > 0 || stateSynced)
				}
				// else {
				// should only happen during testing
				// }

				break FOR_LOOP
			}

		case <-trySyncTicker.C: // chan time
			select {
			case didProcessCh <- struct{}{}:
			default:
			}

		case <-didProcessCh:

			first, second := bcR.pool.PeekTwoBlocks()
			if first == nil || second == nil {
				// We need both to sync the first block.
				continue FOR_LOOP
			} else {
				// Try again quickly next loop.
				didProcessCh <- struct{}{}
			}

			firstParts := first.MakePartSet(types.BlockPartSizeBytes)
			firstPartSetHeader := firstParts.Header()
			firstID := types.BlockID{Hash: first.Hash(), PartSetHeader: firstPartSetHeader}
			// Finally, verify the first block using the second's commit
			// NOTE: we can probably make this more efficient, but note that calling
			// first.Hash() doesn't verify the tx contents, so MakePartSet() is
			// currently necessary.
			err := state.Validators.VerifyReply(
				chainID, firstID, first.Height, second.LastReply)
			if err != nil {
				bcR.Logger.Errorw("Error in validation", "err", err)
				peerID := bcR.pool.RedoRequest(first.Height)
				peer := bcR.Switch.Peers().Get(peerID)
				if peer != nil {
					// NOTE: we've already removed the peer's request, but we
					// still need to clean up the rest.
					bcR.Switch.StopPeerForError(peer, fmt.Errorf("blockchainReactor validation error: %v", err))
				}
				peerID2 := bcR.pool.RedoRequest(second.Height)
				peer2 := bcR.Switch.Peers().Get(peerID2)
				if peer2 != nil && peer2 != peer {
					// NOTE: we've already removed the peer's request, but we
					// still need to clean up the rest.
					bcR.Switch.StopPeerForError(peer2, fmt.Errorf("blockchainReactor validation error: %v", err))
				}
				continue FOR_LOOP
			} else {
				bcR.pool.PopRequest()

				bcR.store.SaveBlock(first, firstParts, second.LastReply)

				var err error
				state, err = bcR.blockExec.ApplyBlock(state, firstID, first)
				if err != nil {
					panic(fmt.Sprintf("Failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
				}
				blocksSynced++

				if blocksSynced%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					bcR.Logger.Infow("Fast Sync Rate", "height", bcR.pool.height,
						"max_peer_height", bcR.pool.MaxPeerHeight(), "blocks/s", lastRate)
					lastHundred = time.Now()
				}
			}
			continue FOR_LOOP

		case <-bcR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `BlockStore` base and height.
func (bcR *BlockchainReactor) BroadcastStatusRequest() error {
	bm, err := EncodeMsg(&protoblockchain.StatusRequest{})
	if err != nil {
		bcR.Logger.Errorw("could not convert msg to proto", "err", err)
		return fmt.Errorf("could not convert msg to proto: %w", err)
	}

	bcR.Switch.Broadcast(BlockchainChannel, bm)

	return nil
}

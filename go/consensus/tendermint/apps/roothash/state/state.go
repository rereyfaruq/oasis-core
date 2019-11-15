package state

import (
	"errors"

	"github.com/tendermint/iavl"

	"github.com/oasislabs/oasis-core/go/common/cbor"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/keyformat"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint/abci"
	registry "github.com/oasislabs/oasis-core/go/registry/api"
	roothash "github.com/oasislabs/oasis-core/go/roothash/api"
	"github.com/oasislabs/oasis-core/go/roothash/api/block"
)

var (
	// runtimeKeyFmt is the key format used for per-runtime roothash state.
	//
	// Value is CBOR-serialized runtime state.
	runtimeKeyFmt = keyformat.New(0x20, &signature.PublicKey{})
	// parametersKeyFmt is the key format used for consensus parameters.
	//
	// Value is CBOR-serialized roothash.ConsensusParameters.
	parametersKeyFmt = keyformat.New(0x21)

	_ cbor.Marshaler   = (*RuntimeState)(nil)
	_ cbor.Unmarshaler = (*RuntimeState)(nil)
)

type RuntimeState struct {
	Runtime      *registry.Runtime `json:"runtime"`
	CurrentBlock *block.Block      `json:"current_block"`
	GenesisBlock *block.Block      `json:"genesis_block"`
	Round        *Round            `json:"round"`
	Timer        abci.Timer        `json:"timer"`
}

func (s *RuntimeState) MarshalCBOR() []byte {
	return cbor.Marshal(s)
}

func (s *RuntimeState) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, s)
}

type ImmutableState struct {
	*abci.ImmutableState
}

func NewImmutableState(state *abci.ApplicationState, version int64) (*ImmutableState, error) {
	inner, err := abci.NewImmutableState(state, version)
	if err != nil {
		return nil, err
	}

	return &ImmutableState{inner}, nil
}

func (s *ImmutableState) RuntimeState(id signature.PublicKey) (*RuntimeState, error) {
	_, raw := s.Snapshot.Get(runtimeKeyFmt.Encode(&id))
	if raw == nil {
		return nil, nil
	}

	var state RuntimeState
	err := state.UnmarshalCBOR(raw)
	return &state, err
}

func (s *ImmutableState) Runtimes() []*RuntimeState {
	var runtimes []*RuntimeState
	s.Snapshot.IterateRange(
		runtimeKeyFmt.Encode(),
		nil,
		true,
		func(key, value []byte) bool {
			if !runtimeKeyFmt.Decode(key) {
				return true
			}

			var state RuntimeState
			cbor.MustUnmarshal(value, &state)

			runtimes = append(runtimes, &state)
			return false
		},
	)

	return runtimes
}

func (s *ImmutableState) ConsensusParameters() (*roothash.ConsensusParameters, error) {
	_, raw := s.Snapshot.Get(parametersKeyFmt.Encode())
	if raw == nil {
		return nil, errors.New("tendermint/roothash: expected consensus parameters to be present in app state")
	}

	var params roothash.ConsensusParameters
	err := cbor.Unmarshal(raw, &params)
	return &params, err
}

type MutableState struct {
	*ImmutableState

	tree *iavl.MutableTree
}

func NewMutableState(tree *iavl.MutableTree) *MutableState {
	inner := &abci.ImmutableState{Snapshot: tree.ImmutableTree}

	return &MutableState{
		ImmutableState: &ImmutableState{inner},
		tree:           tree,
	}
}

func (s *MutableState) SetRuntimeState(state *RuntimeState) {
	s.tree.Set(runtimeKeyFmt.Encode(&state.Runtime.ID), state.MarshalCBOR())
}

func (s *MutableState) SetConsensusParameters(params *roothash.ConsensusParameters) {
	s.tree.Set(parametersKeyFmt.Encode(), cbor.Marshal(params))
}

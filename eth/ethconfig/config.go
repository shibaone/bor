// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package ethconfig contains the configuration of the ETH and LES protocols.
package ethconfig

import (
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/bor"
	"github.com/ethereum/go-ethereum/consensus/bor/contract"
	"github.com/ethereum/go-ethereum/consensus/bor/heimdall" //nolint:typecheck
	"github.com/ethereum/go-ethereum/consensus/bor/heimdall/span"
	"github.com/ethereum/go-ethereum/consensus/bor/heimdallapp"
	"github.com/ethereum/go-ethereum/consensus/bor/heimdallgrpc"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool/blobpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/params"
)

// FullNodeGPO contains default gasprice oracle settings for full node.
var FullNodeGPO = gasprice.Config{
	Blocks:           20,
	Percentile:       60,
	MaxHeaderHistory: 1024,
	MaxBlockHistory:  1024,
	MaxPrice:         gasprice.DefaultMaxPrice,
	IgnorePrice:      gasprice.DefaultIgnorePrice,
}

// LightClientGPO contains default gasprice oracle settings for light client.
var LightClientGPO = gasprice.Config{
	Blocks:           2,
	Percentile:       60,
	MaxHeaderHistory: 300,
	MaxBlockHistory:  5,
	MaxPrice:         gasprice.DefaultMaxPrice,
	IgnorePrice:      gasprice.DefaultIgnorePrice,
}

// Defaults contains default settings for use on the Ethereum main net.
var Defaults = Config{
	SyncMode:           downloader.SnapSync,
	NetworkId:          1,
	TxLookupLimit:      2350000,
	LightPeers:         100,
	DatabaseCache:      512,
	TrieCleanCache:     154,
	TrieDirtyCache:     256,
	TrieTimeout:        60 * time.Minute,
	SnapshotCache:      102,
	FilterLogCacheSize: 32,
	Miner:              miner.DefaultConfig,
	TxPool:             legacypool.DefaultConfig,
	BlobPool:           blobpool.DefaultConfig,
	RPCGasCap:          50000000,
	RPCEVMTimeout:      5 * time.Second,
	GPO:                FullNodeGPO,
	RPCTxFeeCap:        1, // 1 ether
}

//go:generate go run github.com/fjl/gencodec -type Config -formats toml -out gen_config.go

// Config contains configuration options for of the ETH and LES protocols.
type Config struct {
	// The genesis block, which is inserted if the database is empty.
	// If nil, the Ethereum main net block is used.
	Genesis *core.Genesis `toml:",omitempty"`

	// Protocol options
	NetworkId uint64 // Network ID to use for selecting peers to connect to
	SyncMode  downloader.SyncMode

	// This can be set to list of enrtree:// URLs which will be queried for
	// for nodes to connect to.
	EthDiscoveryURLs  []string
	SnapDiscoveryURLs []string

	NoPruning  bool // Whether to disable pruning and flush everything to disk
	NoPrefetch bool // Whether to disable prefetching and only load state on demand

	TxLookupLimit uint64 `toml:",omitempty"` // The maximum number of blocks from head whose tx indices are reserved.

	// RequiredBlocks is a set of block number -> hash mappings which must be in the
	// canonical chain of all remote peers. Setting the option makes geth verify the
	// presence of these blocks for every new peer connection.
	RequiredBlocks map[uint64]common.Hash `toml:"-"`

	// Light client options
	LightServ        int  `toml:",omitempty"` // Maximum percentage of time allowed for serving LES requests
	LightIngress     int  `toml:",omitempty"` // Incoming bandwidth limit for light servers
	LightEgress      int  `toml:",omitempty"` // Outgoing bandwidth limit for light servers
	LightPeers       int  `toml:",omitempty"` // Maximum number of LES client peers
	LightNoPrune     bool `toml:",omitempty"` // Whether to disable light chain pruning
	LightNoSyncServe bool `toml:",omitempty"` // Whether to serve light clients before syncing

	// Database options
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int
	DatabaseFreezer    string

	// Database - LevelDB options
	LevelDbCompactionTableSize           uint64
	LevelDbCompactionTableSizeMultiplier float64
	LevelDbCompactionTotalSize           uint64
	LevelDbCompactionTotalSizeMultiplier float64

	TrieCleanCache int
	TrieDirtyCache int
	TrieTimeout    time.Duration
	SnapshotCache  int
	Preimages      bool
	TriesInMemory  uint64

	// This is the number of blocks for which logs will be cached in the filter system.
	FilterLogCacheSize int

	// Mining options
	Miner miner.Config

	// Transaction pool options
	TxPool   legacypool.Config
	BlobPool blobpool.Config

	// Gas Price Oracle options
	GPO gasprice.Config

	// Enables tracking of SHA3 preimages in the VM
	EnablePreimageRecording bool

	// Miscellaneous options
	DocRoot string `toml:"-"`

	// RPCGasCap is the global gas cap for eth-call variants.
	RPCGasCap uint64

	// Maximum size (in bytes) a result of an rpc request could have
	RPCReturnDataLimit uint64

	// RPCEVMTimeout is the global timeout for eth-call.
	RPCEVMTimeout time.Duration

	// RPCTxFeeCap is the global transaction fee(price * gaslimit) cap for
	// send-transaction variants. The unit is ether.
	RPCTxFeeCap float64

	// OverrideCancun (TODO: remove after the fork)
	OverrideCancun *big.Int `toml:",omitempty"`

	// URL to connect to Heimdall node
	HeimdallURL string

	// No heimdall service
	WithoutHeimdall bool

	// Address to connect to Heimdall gRPC server
	HeimdallgRPCAddress string

	// Run heimdall service as a child process
	RunHeimdall bool

	// Arguments to pass to heimdall service
	RunHeimdallArgs string

	// Use child heimdall process to fetch data, Only works when RunHeimdall is true
	UseHeimdallApp bool

	// Bor logs flag
	BorLogs bool

	// Parallel EVM (Block-STM) related config
	ParallelEVM core.ParallelEVMConfig `toml:",omitempty"`

	// Develop Fake Author mode to produce blocks without authorisation
	DevFakeAuthor bool `hcl:"devfakeauthor,optional" toml:"devfakeauthor,optional"`

	// OverrideVerkle (TODO: remove after the fork)
	OverrideVerkle *big.Int `toml:",omitempty"`

	// EnableBlockTracking allows logging of information collected while tracking block lifecycle
	EnableBlockTracking bool
}

// CreateConsensusEngine creates a consensus engine for the given chain configuration.
func CreateConsensusEngine(chainConfig *params.ChainConfig, ethConfig *Config, db ethdb.Database, blockchainAPI *ethapi.BlockChainAPI) (consensus.Engine, error) {
	// nolint:nestif
	if chainConfig.Clique != nil {
		return beacon.New(clique.New(chainConfig.Clique, db)), nil
	} else if chainConfig.Bor != nil && chainConfig.Bor.ValidatorContract != "" {
		// If Matic bor consensus is requested, set it up
		// In order to pass the ethereum transaction tests, we need to set the burn contract which is in the bor config
		// Then, bor != nil will also be enabled for ethash and clique. Only enable Bor for real if there is a validator contract present.
		genesisContractsClient := contract.NewGenesisContractsClient(chainConfig, chainConfig.Bor.ValidatorContract, chainConfig.Bor.StateReceiverContract, blockchainAPI)
		spanner := span.NewChainSpanner(blockchainAPI, contract.ValidatorSet(), chainConfig, common.HexToAddress(chainConfig.Bor.ValidatorContract))

		if ethConfig.WithoutHeimdall {
			return bor.New(chainConfig, db, blockchainAPI, spanner, nil, genesisContractsClient, ethConfig.DevFakeAuthor), nil
		} else {
			if ethConfig.DevFakeAuthor {
				log.Warn("Sanitizing DevFakeAuthor", "Use DevFakeAuthor with", "--bor.withoutheimdall")
			}

			var heimdallClient bor.IHeimdallClient
			if ethConfig.RunHeimdall && ethConfig.UseHeimdallApp {
				heimdallClient = heimdallapp.NewHeimdallAppClient()
			} else if ethConfig.HeimdallgRPCAddress != "" {
				heimdallClient = heimdallgrpc.NewHeimdallGRPCClient(ethConfig.HeimdallgRPCAddress)
			} else {
				heimdallClient = heimdall.NewHeimdallClient(ethConfig.HeimdallURL)
			}

			return bor.New(chainConfig, db, blockchainAPI, spanner, heimdallClient, genesisContractsClient, false), nil
		}
	}
	if !chainConfig.TerminalTotalDifficultyPassed {
		return nil, errors.New("ethash is only supported as a historical component of already merged networks")
	}
	return beacon.New(ethash.NewFaker()), nil
}

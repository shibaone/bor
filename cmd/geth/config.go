// Copyright 2017 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/external"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/accounts/scwallet"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/internal/version"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

var (
	dumpConfigCommand = &cli.Command{
		Action:      dumpConfig,
		Name:        "dumpconfig",
		Usage:       "Export configuration values in a TOML format",
		ArgsUsage:   "<dumpfile (optional)>",
		Flags:       flags.Merge(nodeFlags, rpcFlags),
		Description: `Export configuration values in TOML format (to stdout by default).`,
	}

	configFileFlag = &cli.StringFlag{
		Name:     "config",
		Usage:    "TOML configuration file",
		Category: flags.EthCategory,
	}
)

type ethstatsConfig struct {
	URL string `toml:",omitempty"`
}

type gethConfig struct {
	Eth      ethconfig.Config
	Node     node.Config
	Ethstats ethstatsConfig
	Metrics  metrics.Config
}

func loadConfig(file string, cfg *gethConfig) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	tomlData := string(data)
	if _, err = toml.Decode(tomlData, &cfg); err != nil {
		return err
	}

	return nil
}

func defaultNodeConfig() node.Config {
	git, _ := version.VCS()
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(git.Commit, git.Date)
	cfg.HTTPModules = append(cfg.HTTPModules, "eth")
	cfg.WSModules = append(cfg.WSModules, "eth")
	cfg.IPCPath = clientIdentifier + ".ipc"

	return cfg
}

// loadBaseConfig loads the gethConfig based on the given command line
// parameters and config file.
func loadBaseConfig(ctx *cli.Context) gethConfig {
	// Load defaults.
	cfg := gethConfig{
		Eth:     ethconfig.Defaults,
		Node:    defaultNodeConfig(),
		Metrics: metrics.DefaultConfig,
	}

	// Load config file.
	if file := ctx.String(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	if ctx.IsSet(utils.MumbaiFlag.Name) {
		setDefaultMumbaiGethConfig(ctx, &cfg)
	}

	if ctx.IsSet(utils.BorMainnetFlag.Name) {
		setDefaultBorMainnetGethConfig(ctx, &cfg)
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	return cfg
}

// makeConfigNode loads geth configuration and creates a blank node instance.
func makeConfigNode(ctx *cli.Context) (*node.Node, gethConfig) {
	cfg := loadBaseConfig(ctx)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}
	// Node doesn't by default populate account manager backends
	if err := setAccountManagerBackends(stack.Config(), stack.AccountManager(), stack.KeyStoreDir()); err != nil {
		utils.Fatalf("Failed to set account manager backends: %v", err)
	}

	utils.SetEthConfig(ctx, stack, &cfg.Eth)

	if ctx.IsSet(utils.EthStatsURLFlag.Name) {
		cfg.Ethstats.URL = ctx.String(utils.EthStatsURLFlag.Name)
	}

	applyMetricConfig(ctx, &cfg)

	// Set Bor config flags
	utils.SetBorConfig(ctx, &cfg.Eth)

	return stack, cfg
}

// makeFullNode loads geth configuration and creates the Ethereum backend.
func makeFullNode(ctx *cli.Context) (*node.Node, ethapi.Backend) {
	stack, cfg := makeConfigNode(ctx)
	if ctx.IsSet(utils.OverrideCancun.Name) {
		v := ctx.Int64(utils.OverrideCancun.Name)
		cfg.Eth.OverrideCancun = new(big.Int).SetInt64(v)
	}
	if ctx.IsSet(utils.OverrideVerkle.Name) {
		v := ctx.Int64(utils.OverrideVerkle.Name)
		cfg.Eth.OverrideVerkle = new(big.Int).SetInt64(v)
	}

	backend, eth := utils.RegisterEthService(stack, &cfg.Eth)

	// Configure log filter RPC API.
	filterSystem := utils.RegisterFilterAPI(stack, backend, &cfg.Eth)

	// Configure GraphQL if requested.
	if ctx.IsSet(utils.GraphQLEnabledFlag.Name) {
		utils.RegisterGraphQLService(stack, backend, filterSystem, &cfg.Node)
	}

	// Add the Ethereum Stats daemon if requested.
	if cfg.Ethstats.URL != "" {
		utils.RegisterEthStatsService(stack, backend, cfg.Ethstats.URL)
	}

	// Configure full-sync tester service if requested
	if ctx.IsSet(utils.SyncTargetFlag.Name) && cfg.Eth.SyncMode == downloader.FullSync {
		utils.RegisterFullSyncTester(stack, eth, ctx.Path(utils.SyncTargetFlag.Name))
	}

	// Start the dev mode if requested, or launch the engine API for
	// interacting with external consensus client.
	if ctx.IsSet(utils.DeveloperFlag.Name) {
		simBeacon, err := catalyst.NewSimulatedBeacon(ctx.Uint64(utils.DeveloperPeriodFlag.Name), eth)
		if err != nil {
			utils.Fatalf("failed to register dev mode catalyst service: %v", err)
		}
		catalyst.RegisterSimulatedBeaconAPIs(stack, simBeacon)
		stack.RegisterLifecycle(simBeacon)
	} else if cfg.Eth.SyncMode != downloader.LightSync {
		err := catalyst.Register(stack, eth)
		if err != nil {
			utils.Fatalf("failed to register catalyst service: %v", err)
		}
	}
	return stack, backend
}

// dumpConfig is the dumpconfig command.
func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)

	if cfg.Eth.Genesis != nil {
		cfg.Eth.Genesis = nil
	}

	if err := toml.NewEncoder(os.Stdout).Encode(&cfg); err != nil {
		return err
	}

	return nil
}

func applyMetricConfig(ctx *cli.Context, cfg *gethConfig) {
	if ctx.IsSet(utils.MetricsEnabledFlag.Name) {
		cfg.Metrics.Enabled = ctx.Bool(utils.MetricsEnabledFlag.Name)
	}

	if ctx.IsSet(utils.MetricsEnabledExpensiveFlag.Name) {
		cfg.Metrics.EnabledExpensive = ctx.Bool(utils.MetricsEnabledExpensiveFlag.Name)
	}

	if ctx.IsSet(utils.MetricsHTTPFlag.Name) {
		cfg.Metrics.HTTP = ctx.String(utils.MetricsHTTPFlag.Name)
	}

	if ctx.IsSet(utils.MetricsPortFlag.Name) {
		cfg.Metrics.Port = ctx.Int(utils.MetricsPortFlag.Name)
	}

	if ctx.IsSet(utils.MetricsEnableInfluxDBFlag.Name) {
		cfg.Metrics.EnableInfluxDB = ctx.Bool(utils.MetricsEnableInfluxDBFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBEndpointFlag.Name) {
		cfg.Metrics.InfluxDBEndpoint = ctx.String(utils.MetricsInfluxDBEndpointFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBDatabaseFlag.Name) {
		cfg.Metrics.InfluxDBDatabase = ctx.String(utils.MetricsInfluxDBDatabaseFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBUsernameFlag.Name) {
		cfg.Metrics.InfluxDBUsername = ctx.String(utils.MetricsInfluxDBUsernameFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBPasswordFlag.Name) {
		cfg.Metrics.InfluxDBPassword = ctx.String(utils.MetricsInfluxDBPasswordFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBTagsFlag.Name) {
		cfg.Metrics.InfluxDBTags = ctx.String(utils.MetricsInfluxDBTagsFlag.Name)
	}

	if ctx.IsSet(utils.MetricsEnableInfluxDBV2Flag.Name) {
		cfg.Metrics.EnableInfluxDBV2 = ctx.Bool(utils.MetricsEnableInfluxDBV2Flag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBTokenFlag.Name) {
		cfg.Metrics.InfluxDBToken = ctx.String(utils.MetricsInfluxDBTokenFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBBucketFlag.Name) {
		cfg.Metrics.InfluxDBBucket = ctx.String(utils.MetricsInfluxDBBucketFlag.Name)
	}

	if ctx.IsSet(utils.MetricsInfluxDBOrganizationFlag.Name) {
		cfg.Metrics.InfluxDBOrganization = ctx.String(utils.MetricsInfluxDBOrganizationFlag.Name)
	}
}

func setAccountManagerBackends(conf *node.Config, am *accounts.Manager, keydir string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP

	if conf.UseLightweightKDF {
		scryptN = keystore.LightScryptN
		scryptP = keystore.LightScryptP
	}

	// Assemble the supported backends
	if len(conf.ExternalSigner) > 0 {
		log.Info("Using external signer", "url", conf.ExternalSigner)
		if extBackend, err := external.NewExternalBackend(conf.ExternalSigner); err == nil {
			am.AddBackend(extBackend)
			return nil
		} else {
			return fmt.Errorf("error connecting to external signer: %v", err)
		}
	}

	// For now, we're using EITHER external signer OR local signers.
	// If/when we implement some form of lockfile for USB and keystore wallets,
	// we can have both, but it's very confusing for the user to see the same
	// accounts in both externally and locally, plus very racey.
	am.AddBackend(keystore.NewKeyStore(keydir, scryptN, scryptP))

	if conf.USB {
		// Start a USB hub for Ledger hardware wallets
		if ledgerhub, err := usbwallet.NewLedgerHub(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start Ledger hub, disabling: %v", err))
		} else {
			am.AddBackend(ledgerhub)
		}
		// Start a USB hub for Trezor hardware wallets (HID version)
		if trezorhub, err := usbwallet.NewTrezorHubWithHID(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start HID Trezor hub, disabling: %v", err))
		} else {
			am.AddBackend(trezorhub)
		}
		// Start a USB hub for Trezor hardware wallets (WebUSB version)
		if trezorhub, err := usbwallet.NewTrezorHubWithWebUSB(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start WebUSB Trezor hub, disabling: %v", err))
		} else {
			am.AddBackend(trezorhub)
		}
	}

	if len(conf.SmartCardDaemonPath) > 0 {
		// Start a smart card hub
		if schub, err := scwallet.NewHub(conf.SmartCardDaemonPath, scwallet.Scheme, keydir); err != nil {
			log.Warn(fmt.Sprintf("Failed to start smart card hub, disabling: %v", err))
		} else {
			am.AddBackend(schub)
		}
	}

	return nil
}

func setDefaultMumbaiGethConfig(ctx *cli.Context, config *gethConfig) {
	config.Node.P2P.ListenAddr = fmt.Sprintf(":%d", 30303)
	config.Node.HTTPHost = "0.0.0.0"
	config.Node.HTTPVirtualHosts = []string{"*"}
	config.Node.HTTPCors = []string{"*"}
	config.Node.HTTPPort = 8545
	config.Node.IPCPath = utils.MakeDataDir(ctx) + "/bor.ipc"
	config.Node.HTTPModules = []string{"eth", "net", "web3", "txpool", "bor"}
	config.Eth.SyncMode = downloader.FullSync
	config.Eth.NetworkId = 80001
	config.Eth.Miner.GasCeil = 20000000
	//--miner.gastarget is deprecated, No longed used
	config.Eth.TxPool.NoLocals = true
	config.Eth.TxPool.AccountSlots = 16
	config.Eth.TxPool.GlobalSlots = 131072
	config.Eth.TxPool.AccountQueue = 64
	config.Eth.TxPool.GlobalQueue = 131072
	config.Eth.TxPool.Lifetime = 90 * time.Minute
	config.Node.P2P.MaxPeers = 50
	config.Metrics.Enabled = true
	// --pprof is enabled in 'internal/debug/flags.go'
}

func setDefaultBorMainnetGethConfig(ctx *cli.Context, config *gethConfig) {
	config.Node.P2P.ListenAddr = fmt.Sprintf(":%d", 30303)
	config.Node.HTTPHost = "0.0.0.0"
	config.Node.HTTPVirtualHosts = []string{"*"}
	config.Node.HTTPCors = []string{"*"}
	config.Node.HTTPPort = 8545
	config.Node.IPCPath = utils.MakeDataDir(ctx) + "/bor.ipc"
	config.Node.HTTPModules = []string{"eth", "net", "web3", "txpool", "bor"}
	config.Eth.SyncMode = downloader.FullSync
	config.Eth.NetworkId = 137
	config.Eth.Miner.GasCeil = 20000000
	//--miner.gastarget is deprecated, No longed used
	config.Eth.TxPool.NoLocals = true
	config.Eth.TxPool.AccountSlots = 16
	config.Eth.TxPool.GlobalSlots = 131072
	config.Eth.TxPool.AccountQueue = 64
	config.Eth.TxPool.GlobalQueue = 131072
	config.Eth.TxPool.Lifetime = 90 * time.Minute
	config.Node.P2P.MaxPeers = 50
	config.Metrics.Enabled = true
	// --pprof is enabled in 'internal/debug/flags.go'
}

package commands

import (
	"fmt"
	cfg "github.com/232425wxy/BFT/config"
	"github.com/232425wxy/BFT/gossip"
	srrand "github.com/232425wxy/BFT/libs/rand"
	srtime "github.com/232425wxy/BFT/libs/time"
	"github.com/232425wxy/BFT/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net"
	"os"
	"path/filepath"
	"strings"
)

var (
	nValidators    int
	initialHeight  int64
	configFile     string
	outputDir      string
	nodeDirPrefix  string

	populatePersistentPeers bool
	startingIPAddress       string
	p2pPort                 int
)

const (
	nodeDirPerm = 0755
)

func init() {
	TestnetFilesCmd.Flags().IntVar(&nValidators, "v", 4, "number of validators to initialize the testnet with")
	TestnetFilesCmd.Flags().StringVar(&configFile, "config", "", "config file to use (note some options may be overwritten)")
	TestnetFilesCmd.Flags().StringVar(&outputDir, "o", "./mytestnet", "directory to store initialization data for the testnet")
	TestnetFilesCmd.Flags().StringVar(&nodeDirPrefix, "node-dir-prefix", "node", "prefix the directory name for each node with (node results in node0, node1, ...)")
	TestnetFilesCmd.Flags().Int64Var(&initialHeight, "initial-height", 0, "initial height of the first block")
	TestnetFilesCmd.Flags().BoolVar(&populatePersistentPeers, "populate-persistent-peers", true, "update config of each node with the list of persistent peers build using either hostname-prefix or starting-ip-address")
	TestnetFilesCmd.Flags().StringVar(&startingIPAddress, "starting-ip-address", "", "starting IP address (\"192.168.0.1\" results in persistent peers list ID0@192.168.0.1:26656, ID1@192.168.0.2:26656, ...)")
	TestnetFilesCmd.Flags().IntVar(&p2pPort, "p2p-port", 36656, "P2P Port")
}

// TestnetFilesCmd allows initialisation of files for a SRBFT testnet.
var TestnetFilesCmd = &cobra.Command{
	Use:   "testnet",
	Short: "Initialize files for a SRBFT testnet",
	Long: `testnet will create "v" + "n" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).

Note, strict routability for addresses is turned off in the config file.

Optionally, it will fill in persistent_peers list in config file using either hostnames or IPs.

Example:

	BFT testnet --v 4 --o ./output --populate-persistent-peers --starting-ip-address 192.168.10.2
	`,
	RunE: testnetFiles,
}

func testnetFiles(cmd *cobra.Command, args []string) error {

	config := cfg.DefaultConfig()
	// overwrite default config if set and valid
	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
		if err := viper.Unmarshal(config); err != nil {
			return err
		}
		if err := config.ValidateBasic(); err != nil {
			return err
		}
	}

	genVals := make([]types.GenesisValidator, nValidators)

	for i := 0; i < nValidators; i++ {
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)  // nodeDirPrefix: node, i: 0 => nodeDirName: node0
		nodeDir := filepath.Join(outputDir, nodeDirName)  // outputDir: .
		config.SetRoot(nodeDir)

		err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
		err = os.MkdirAll(filepath.Join(nodeDir, "data"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		if err := initFilesWithConfig(config); err != nil {
			return err
		}

		pvKeyFile := filepath.Join(nodeDir, config.BaseConfig.PrivValidatorKey)
		pvStateFile := filepath.Join(nodeDir, config.BaseConfig.PrivValidatorState)
		pv := types.LoadFilePV(pvKeyFile, pvStateFile)

		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
		genVals[i] = types.GenesisValidator{
			Address: pubKey.Address(),
			PubKey:  pubKey,
			Power:   1,
			Name:    nodeDirName,
		}
	}


	// Generate genesis doc from generated validators
	genDoc := &types.GenesisDoc{
		ChainID:         "chain-" + srrand.Str(6),
		ConsensusParams: types.DefaultConsensusParams(),
		GenesisTime:     srtime.Now(),
		InitialHeight:   initialHeight,
		Validators:      genVals,
	}

	// Write genesis file.
	for i := 0; i < nValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		if err := genDoc.SaveAs(filepath.Join(nodeDir, config.BaseConfig.Genesis)); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Gather persistent peer addresses.
	var (
		persistentPeers string
		err             error
	)
	if populatePersistentPeers {
		persistentPeers, err = persistentPeersString(config)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Overwrite default config.
	for i := 0; i < nValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		config.SetRoot(nodeDir)
		config.P2P.AddrBookStrict = false
		if populatePersistentPeers {
			config.P2P.PersistentPeers = persistentPeers
		}
		cfg.WriteConfigFile(filepath.Join(nodeDir, "config", "config.toml"), config)
	}

	fmt.Printf("Successfully initialized %v node directories\n", nValidators)
	return nil
}

func hostnameOrIP(i int) string {
	ip := net.ParseIP(startingIPAddress)
	ip = ip.To4()
	if ip == nil {
		fmt.Printf("%v: non ipv4 address\n", startingIPAddress)
		os.Exit(1)
	}

	for j := 0; j < i; j++ {
		ip[3]++
	}
	return ip.String()
}

func persistentPeersString(config *cfg.Config) (string, error) {
	persistentPeers := make([]string, nValidators)
	for i := 0; i < nValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		config.SetRoot(nodeDir)
		nodeKey, err := gossip.LoadNodeKey(config.NodeKeyFile())
		if err != nil {
			return "", err
		}
		persistentPeers[i] = gossip.IDAddressString(nodeKey.ID(), fmt.Sprintf("%s:%d", hostnameOrIP(i), p2pPort))
	}
	return strings.Join(persistentPeers, ","), nil
}

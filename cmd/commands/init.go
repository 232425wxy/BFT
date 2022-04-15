package commands

import (
	"BFT/gossip"
	srtime "BFT/libs/time"
	"fmt"
	"github.com/spf13/cobra"
	cfg "BFT/config"
	sros "BFT/libs/os"
	srrand "BFT/libs/rand"
	"BFT/types"
)

// InitFilesCmd initialises a fresh SRBFT Core instance.
var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize SRBFT",
	RunE:  initFiles,
}

func initFiles(cmd *cobra.Command, args []string) error {
	return initFilesWithConfig(config)
}

func initFilesWithConfig(config *cfg.Config) error {
	// 获取将要存储 privValidator 信息的文件地址
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	fmt.Println(privValKeyFile, privValStateFile)
	var pv *types.FilePV
	if sros.FileExists(privValKeyFile) {
		pv = types.LoadFilePV(privValKeyFile, privValStateFile)
		logger.Info("Found private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv = types.GenFilePV(privValKeyFile, privValStateFile)
		pv.Save()
		logger.Info("Generated private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}
	nodeKeyFile := config.NodeKeyFile()
	if sros.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := gossip.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if sros.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID:         fmt.Sprintf("chain-%v", srrand.Str(6)),
			GenesisTime:     srtime.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
		}
		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
		genDoc.Validators = []types.GenesisValidator{{
			Address: pubKey.Address(),
			PubKey:  pubKey,
			Power:   1,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}

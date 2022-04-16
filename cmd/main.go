package main

import (
	"os"
	"path/filepath"

	cmd "github.com/232425wxy/BFT/cmd/commands"
	cfg "github.com/232425wxy/BFT/config"
	"github.com/232425wxy/BFT/libs/cli"
	nm "github.com/232425wxy/BFT/node"
)

func main() {
	nodeFunc := nm.DefaultNewNode
	rootCmd := cmd.RootCmd
	rootCmd.AddCommand(
		cmd.InitFilesCmd,
		cmd.TestnetFilesCmd,
		cli.NewCompletionCmd(rootCmd, true),
	)

	rootCmd.AddCommand(cmd.NewRunNodeCmd(nodeFunc))

	command := cli.PrepareBaseCmd(rootCmd, "CR", os.ExpandEnv(filepath.Join("$HOME", cfg.DefaultHomeDir)))
	if err := command.Execute(); err != nil {
		panic(err)
	}
}

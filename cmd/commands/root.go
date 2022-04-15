package commands

import (
	cfg "BFT/config"
	"BFT/libs/log"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	config = cfg.DefaultConfig()
	logger = log.NewCRLogger(config.LogLevel)
)

func init() {
	registerFlagsRootCmd(RootCmd)
}

func registerFlagsRootCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().String("log_level", config.LogLevel, "log level")
}

// ParseConfig retrieves the default environment configuration,
// sets up the SRBFT root and ensures that the root exists
func ParseConfig() (*cfg.Config, error) {
	conf := cfg.DefaultConfig()
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	conf.SetRoot(conf.RootDir)
	cfg.EnsureRoot(conf.RootDir)
	if err := conf.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error in config file: %v", err)
	}
	return conf, nil
}

// RootCmd is the root command for SRBFT core.
var RootCmd = &cobra.Command{
	Use:   "SRBFT",
	Short: "BFT state machine replication for applications in any programming languages",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err = ParseConfig()
		if err != nil {
			return err
		}

		return nil
	},
}

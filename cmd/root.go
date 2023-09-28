/*
Copyright © 2023 Glif LTD

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	jnal "github.com/glifio/cli/journal"
	"github.com/glifio/cli/journal/fsjournal"
	"github.com/glifio/cli/util"
	"github.com/glifio/go-pools/constants"
	"github.com/glifio/go-pools/deploy"
	"github.com/glifio/go-pools/sdk"
	types "github.com/glifio/go-pools/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
)

var cfgDir string
var useCalibnet bool // only set in root_calibnet.go
var chainID int64 = constants.MainnetChainID
var PoolsSDK types.PoolsSDK
var journal jnal.Journal

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "glif",
}

var rootInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Prints information about the CLI",
	Long:  `Prints information about the CLI`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Config directory: %s\n", cfgDir)
		fmt.Printf("Chain ID: %d\n", chainID)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", "", "config directory")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.AddCommand(rootInfoCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if os.Getenv("GLIF_CONFIG_DIR") != "" {
		cfgDir = os.Getenv("GLIF_CONFIG_DIR")
	}
	if cfgDir != "" {
		viper.AddConfigPath(cfgDir)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		cfgDir = fmt.Sprintf("%s/.glif", home)
		if useCalibnet {
			cfgDir = fmt.Sprintf("%s/.glif/%s", home, "calibnet")
		}

		// Search config in home directory with name ".glif" (without extension).
		viper.AddConfigPath(cfgDir)
		viper.AddConfigPath(".")
	}

	viper.SetConfigType("toml")
	viper.SetConfigName("config")

	var err error
	if journal, err = fsjournal.OpenFSJournal(cfgDir, nil); err != nil {
		logFatal(err)
	}

	util.NewKeyStore(fmt.Sprintf("%s/keystore", cfgDir))

	if err := util.NewKeyStoreLegacy(fmt.Sprintf("%s/keys.toml", cfgDir)); err != nil {
		logFatal(err)
	}

	if err := util.NewAgentStore(fmt.Sprintf("%s/agent.toml", cfgDir)); err != nil {
		logFatal(err)
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logFatalf("No config file found at %s\n", viper.ConfigFileUsed())
		} else if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Fprintln(os.Stderr, "Warning: No config file found.")
		} else {
			logFatalf("Config file error: %v\n", err)
		}
	}

	viper.WatchConfig()

	if slices.Contains(os.Args[1:], "wallet") &&
		(slices.Contains(os.Args[1:], "new") ||
			slices.Contains(os.Args[1:], "new-msig") ||
			slices.Contains(os.Args[1:], "migrate")) {
		// Skip migration check
	} else {
		err = checkWalletMigrated()
		if err != nil {
			logFatal(err)
		}

		err = checkUnencryptedPrivateKeys()
		if err != nil {
			log.Println(err)
		}
	}

	daemonURL := viper.GetString("daemon.rpc-url")
	daemonToken := viper.GetString("daemon.token")
	adoURL := viper.GetString("ado.address")

	if chainID == constants.LocalnetChainID || chainID == constants.AnvilChainID {
		routerAddr := viper.GetString("routes.router")
		router := common.HexToAddress(routerAddr)
		err := sdk.LazyInit(context.Background(), &PoolsSDK, router, adoURL, "Mock", daemonURL, daemonToken)
		if err != nil {
			logFatal(err)
		}
	} else {
		var extern types.Extern
		switch chainID {
		case constants.MainnetChainID:
			extern = deploy.Extern
		case constants.CalibnetChainID:
			extern = deploy.TestExtern
		default:
			logFatalf("Unknown chain id %d", chainID)
		}

		if daemonURL != "" {
			extern.LotusDialAddr = daemonURL
		}
		if daemonToken != "" {
			extern.LotusToken = daemonToken
		}

		if adoURL != "" {
			extern.AdoAddr = adoURL
		}

		sdk, err := sdk.New(context.Background(), big.NewInt(chainID), extern)
		if err != nil {
			logFatalf("Failed to initialize pools sdk %s", err)
		}
		PoolsSDK = sdk
	}
}

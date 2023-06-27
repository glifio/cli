/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/filecoin-project/go-address"
	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	ledgerfil "github.com/whyrusleeping/ledger-filecoin-go"
)

const hdHard = 0x80000000

var filHDBasePath = []uint32{hdHard | 44, hdHard | 461, hdHard, 0}

func panicIfKeyExists(key util.KeyType, addr common.Address, err error) {
	if err != nil {
		logFatal(err)
	}

	if !util.IsZeroAddress(addr) {
		logFatalf("Key already exists for %s", key)
	}
}

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a set of keys",
	Long:  `Creates an owner, an operator, and a requester key and stores the values in $HOME/.config/glif/keys.toml. Note that the owner and requester keys are only applicable to Agents, the operator key is the primary key for interacting with smart contracts.`,
	Run: func(cmd *cobra.Command, args []string) {
		ks := util.KeyStore()

		ownerAddr, _, err := ks.GetAddrs(util.OwnerKey)
		panicIfKeyExists(util.OwnerKey, ownerAddr, err)

		operatorAddr, _, err := ks.GetAddrs(util.OperatorKey)
		panicIfKeyExists(util.OperatorKey, operatorAddr, err)

		requestAddr, _, err := ks.GetAddrs(util.RequestKey)
		panicIfKeyExists(util.RequestKey, requestAddr, err)

		// Create the Ethereum private key
		hardwareWallet, _ := cmd.Flags().GetBool("use-hardware-wallet-for-owner")

		if hardwareWallet {
			fmt.Println("Looking for Ledger hardware wallet...")

			fl, err := ledgerfil.FindLedgerFilecoinApp()
			if err != nil {
				log.Fatalf("finding ledger: %e", err)
			}
			defer fl.Close() // nolint:errcheck

			path := append(append([]uint32(nil), filHDBasePath...), uint32(0))
			_, _, addr, err := fl.GetAddressPubKeySECP256K1(path)
			if err != nil {
				log.Fatalf("getting public key from ledger: %e", err)
			}

			fmt.Printf("creating key: %s, accept the key in ledger device", addr)
			_, _, addr, err = fl.ShowAddressPubKeySECP256K1(path)
			if err != nil {
				log.Fatalf("verifying public key with ledger: %e", err)
			}

			a, err := address.NewFromString(addr)
			if err != nil {
				log.Fatalf("parsing address: %s", err)
			}

			fmt.Println("\nAddress:", a)
			os.Exit(1)
		} else {
			ownerPrivateKey, err := crypto.GenerateKey()
			if err != nil {
				logFatal(err)
			}

			if err := ks.SetKey(util.OwnerKey, ownerPrivateKey); err != nil {
				logFatal(err)
			}
		}

		operatorPrivateKey, err := crypto.GenerateKey()
		if err != nil {
			logFatal(err)
		}

		requestPrivateKey, err := crypto.GenerateKey()
		if err != nil {
			logFatal(err)
		}

		if err := ks.SetKey(util.OperatorKey, operatorPrivateKey); err != nil {
			logFatal(err)
		}

		if err := ks.SetKey(util.RequestKey, requestPrivateKey); err != nil {
			logFatal(err)
		}

		if err := viper.WriteConfig(); err != nil {
			logFatal(err)
		}

		ownerAddr, ownerDelAddr, err := ks.GetAddrs(util.OwnerKey)
		if err != nil {
			logFatal(err)
		}
		operatorAddr, operatorDelAddr, err := ks.GetAddrs(util.OperatorKey)
		if err != nil {
			logFatal(err)
		}
		requestAddr, requestDelAddr, err := ks.GetAddrs(util.RequestKey)
		if err != nil {
			logFatal(err)
		}

		log.Printf("Owner address: %s (ETH), %s (FIL)\n", ownerAddr, ownerDelAddr)
		log.Printf("Operator address: %s (ETH), %s (FIL)\n", operatorAddr, operatorDelAddr)
		log.Printf("Request key: %s (ETH), %s (FIL)\n", requestAddr, requestDelAddr)
		log.Println()
		log.Println("Please make sure to fund your Owner Address with FIL before creating an Agent")
	},
}

func init() {
	walletCmd.AddCommand(newCmd)
	newCmd.Flags().Bool("use-hardware-wallet-for-owner", false, "Use a hardware wallet for the owner (eg. Ledger)")
	// newCmd.Flags().String("ledger-app", "filecoin", "Which Ledger app to use")
	// newCmd.Flags().String("ledger-account", "0", "Which Ledger account to use")
}

/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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

		as := util.AgentStore()
		ks := util.KeyStore()
		ksLegacy := util.KeyStoreLegacy()

		requestAddr, _, err := ksLegacy.GetAddrs(util.RequestKey)
		panicIfKeyExists(util.RequestKey, requestAddr, err)

		useLedger, _ := cmd.Flags().GetBool("ledger")

		var owner accounts.Account

		if !useLedger {
			ownerPassphrase, envSet := os.LookupEnv("GLIF_OWNER_PASSPHRASE")
			if !envSet {
				prompt := &survey.Password{
					Message: "Please type a passphrase to encrypt your owner private key",
				}
				survey.AskOne(prompt, &ownerPassphrase)
			}
			owner, err = ks.NewAccount(ownerPassphrase)
			if err != nil {
				logFatal(err)
			}
		} else {
			fmt.Println("Jim1")
			ledgerhub, err := usbwallet.NewLedgerHub()
			if err != nil {
				logFatal("Ledger not found")
			}
			wallets := ledgerhub.Wallets()
			if len(wallets) == 0 {
				logFatal("No wallets found")
			}
			wallet := wallets[0]
			pathstr := "m/44'/60'/0'/0/0"
			path, _ := accounts.ParseDerivationPath(pathstr)
			fmt.Println("Jim2", path)
			err = wallet.Open("")
			if err == nil {
				owner, err = wallet.Derive(path, false)
			}
			if err != nil {
				fmt.Println(err)
				logFatal("Ledger needs to be in Ethereum app with browser support off")
			}
			fmt.Println("Jim3", owner)
		}

		operatorPassphrase := os.Getenv("GLIF_OPERATOR_PASSPHRASE")
		operator, err := ks.NewAccount(operatorPassphrase)
		if err != nil {
			logFatal(err)
		}

		requestPrivateKey, err := crypto.GenerateKey()
		if err != nil {
			logFatal(err)
		}

		as.Set(string(util.OwnerKey), owner.Address.String())
		as.Set("owner-wallet-url", owner.URL.String())
		as.Set(string(util.OperatorKey), operator.Address.String())

		if err := ksLegacy.SetKey(util.RequestKey, requestPrivateKey); err != nil {
			logFatal(err)
		}

		if err := viper.WriteConfig(); err != nil {
			logFatal(err)
		}

		ownerAddr, ownerDelAddr, err := as.GetAddrs(util.OwnerKey)
		if err != nil {
			logFatal(err)
		}
		operatorAddr, operatorDelAddr, err := as.GetAddrs(util.OperatorKey)
		if err != nil {
			logFatal(err)
		}
		requestAddr, requestDelAddr, err := ksLegacy.GetAddrs(util.RequestKey)
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
	newCmd.Flags().Bool("ledger", false, "Use Ledger hardware wallet for owner")
}

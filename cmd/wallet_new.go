/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"log"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/glifio/cli/util"
	"github.com/glifio/go-wallet-utils/accounts"
	"github.com/glifio/go-wallet-utils/usbwallet"
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

		ownerAddr, _, err := as.GetAddrs(util.OwnerKey)
		panicIfKeyExists(util.OwnerKey, ownerAddr, err)

		operatorAddr, _, err := as.GetAddrs(util.OperatorKey)
		panicIfKeyExists(util.OperatorKey, operatorAddr, err)

		requestAddr, _, err := as.GetAddrs(util.RequestKey)
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
			ksOwner, err := ks.NewAccount(ownerPassphrase)
			if err != nil {
				logFatal(err)
			}
			owner = accounts.Account{EthAccount: ksOwner}
		} else {
			ledgerhub, err := usbwallet.NewLedgerHub()
			if err != nil {
				logFatal("Ledger not found")
			}
			wallets := ledgerhub.Wallets()
			if len(wallets) == 0 {
				logFatal("No wallets found")
			}
			wallet := wallets[0]
			accounts := wallet.Accounts()
			owner = accounts[0]
		}

		operatorPassphrase := os.Getenv("GLIF_OPERATOR_PASSPHRASE")
		operator, err := ks.NewAccount(operatorPassphrase)
		if err != nil {
			logFatal(err)
		}

		requester, err := ks.NewAccount("")
		if err != nil {
			logFatal(err)
		}

		as.Set(string(util.OwnerKey), owner.String())
		as.Set(string(util.OperatorKey), operator.Address.String())
		as.Set(string(util.RequestKey), requester.Address.String())

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
		requestAddr, requestDelAddr, err := as.GetAddrs(util.RequestKey)
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

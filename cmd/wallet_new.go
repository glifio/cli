/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"log"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/filecoin-project/go-address"
	filcrypto "github.com/filecoin-project/go-crypto"
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
	Long:  `Creates an owner, an operator, and a requester key and stores the values in the keystore. Note that the owner and requester keys are only applicable to Agents, the operator key is the primary key for interacting with smart contracts.`,
	Run: func(cmd *cobra.Command, args []string) {
		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			logFatalf("Failed to instantiate eth client %s", err)
		}
		defer closer()

		as := util.AgentStore()
		ks := util.KeyStore()

		ownerAddr, _, err := as.GetAddrs(util.OwnerKey, nil)
		panicIfKeyExists(util.OwnerKey, ownerAddr, err)

		operatorAddr, _, err := as.GetAddrs(util.OperatorKey, nil)
		panicIfKeyExists(util.OperatorKey, operatorAddr, err)

		requestAddr, _, err := as.GetAddrs(util.RequestKey, nil)
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
			as.Set(string(util.OwnerKey), owner.String())
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
			walletAccounts := wallet.Accounts()

			// Note: owner will be an msig, created in a separate step
			ksOwnerProposer, err := ks.NewAccount("")
			if err != nil {
				logFatal(err)
			}
			ownerProposerKeyJSON, err := ks.Export(ksOwnerProposer, "", "")
			if err != nil {
				logFatal(err)
			}
			opk, err := keystore.DecryptKey(ownerProposerKeyJSON, "")
			if err != nil {
				logFatal(err)
			}
			opkPrivateKeyBytes := crypto.FromECDSA(opk.PrivateKey)
			ownerProposerPublicKey := filcrypto.PublicKey(opkPrivateKeyBytes)
			ownerProposerFilAddr, err := address.NewSecp256k1Address(ownerProposerPublicKey)
			if err != nil {
				logFatal(err)
			}
			ownerProposer := accounts.Account{FilAddress: ownerProposerFilAddr}
			as.Set(string(util.OwnerProposerKey), ownerProposer.String())

			ownerApprover := walletAccounts[0]
			as.Set(string(util.OwnerApproverKey), ownerApprover.String())
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

		as.Set(string(util.OperatorKey), operator.Address.String())
		as.Set(string(util.RequestKey), requester.Address.String())

		if err := viper.WriteConfig(); err != nil {
			logFatal(err)
		}

		listAddresses(lapi)
		log.Println()
		log.Println("Please make sure to fund your Owner Address with FIL before creating an Agent")
	},
}

func init() {
	walletCmd.AddCommand(newCmd)
	newCmd.Flags().Bool("ledger", false, "Use Ledger hardware wallet for owner")
}

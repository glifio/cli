/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	ethaccounts "github.com/ethereum/go-ethereum/accounts"
	"github.com/filecoin-project/go-address"
	"github.com/glifio/cli/util"
	"github.com/glifio/go-wallet-utils/accounts"
	"github.com/glifio/go-wallet-utils/usbwallet"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Glif agent",
	Long:  `Spins up a new Agent contract through the Agent Factory, passing the owner, operator, and requestor addresses.`,
	Run: func(cmd *cobra.Command, args []string) {
		as := util.AgentStore()
		ks := util.KeyStore()

		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			logFatalf("Failed to instantiate eth client %s", err)
		}
		defer closer()

		// Check if an agent already exists
		addressStr, err := as.Get("address")
		if err != nil && err != util.ErrKeyNotFound {
			logFatal(err)
		}
		if addressStr != "" {
			logFatalf("Agent already exists: %s", addressStr)
		}

		ownerAddr, ownerFilAddr, err := as.GetAddrs(util.OwnerKey, lapi)
		if err != nil {
			logFatal(err)
		}

		_, proposer, err := as.GetAddrs(util.OwnerProposerKey, lapi)
		if err != nil {
			logFatal(err)
		}

		_, approver, err := as.GetAddrs(util.OwnerApproverKey, lapi)
		if err != nil {
			logFatal(err)
		}

		operatorAddr, _, err := as.GetAddrs(util.OperatorKey, nil)
		if err != nil {
			logFatal(err)
		}

		requestAddr, _, err := as.GetAddrs(util.RequestKey, nil)
		if err != nil {
			logFatal(err)
		}

		var account accounts.Account
		var passphrase string

		if ownerFilAddr.Protocol() == address.Actor {
			// f2 address = msig
			if ownerFilAddr.Empty() {
				logFatal("Owner key not found")
			}

			account = accounts.Account{FilAddress: ownerFilAddr}
		} else if ownerFilAddr.Protocol() == address.SECP256K1 {
			if ownerFilAddr.Empty() {
				logFatal("Owner key not found")
			}

			account = accounts.Account{FilAddress: ownerFilAddr}
		} else {
			account = accounts.Account{EthAccount: ethaccounts.Account{Address: ownerAddr}}
			var envSet bool
			passphrase, envSet = os.LookupEnv("GLIF_OWNER_PASSPHRASE")
			if !envSet {
				prompt := &survey.Password{
					Message: "Owner key passphrase",
				}
				survey.AskOne(prompt, &passphrase)
			}
		}

		backends := []ethaccounts.Backend{}
		backends = append(backends, ks)
		filBackends := []accounts.Backend{}
		if account.IsFil() {
			ledgerhub, err := usbwallet.NewLedgerHub()
			if err != nil {
				logFatal("Ledger not found")
			}
			filBackends = []accounts.Backend{ledgerhub}
		}
		manager := accounts.NewManager(&ethaccounts.Config{InsecureUnlockAllowed: false}, backends, filBackends)

		wallet, err := manager.Find(account)
		if err != nil {
			logFatal(err)
		}

		if util.IsZeroAddress(requestAddr) {
			logFatal("Requester key not found.")
		}

		fmt.Printf("Creating agent, owner %s, operator %s, request %s", ownerAddr, operatorAddr, requestAddr)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		// submit the agent create transaction
		txHash, _, err := PoolsSDK.Act().AgentCreate(
			cmd.Context(),
			ownerAddr,
			operatorAddr,
			requestAddr,
			wallet,
			account,
			passphrase,
			proposer,
			approver,
		)
		if err != nil {
			logFatalf("pools sdk: agent create: %s", err)
		}

		s.Stop()

		fmt.Printf("Agent create transaction submitted: %s\n", txHash)
		fmt.Println("Waiting for confirmation...")

		s.Start()
		// transaction landed on chain or errored
		receipt, err := PoolsSDK.Query().StateWaitReceipt(cmd.Context(), txHash)
		if err != nil {
			logFatalf("pools sdk: query: state wait receipt: %s", err)
		}

		// grab the ID and the address of the agent from the receipt's logs
		addr, id, err := PoolsSDK.Query().AgentAddrIDFromRcpt(cmd.Context(), receipt)
		if err != nil {
			logFatalf("pools sdk: query: agent addr id from receipt: %s", err)
		}

		s.Stop()

		fmt.Printf("Agent created: %s\n", addr.String())
		fmt.Printf("Agent ID: %s\n", id.String())

		as.Set("id", id.String())
		as.Set("address", addr.String())
		as.Set("tx", txHash.String())
	},
}

func init() {
	agentCmd.AddCommand(createCmd)

	createCmd.Flags().String("ownerfile", "", "Owner eth address")
	createCmd.Flags().String("operatorfile", "", "Repayment eth address")
	createCmd.Flags().String("deployerfile", "", "Deployer eth address")
}

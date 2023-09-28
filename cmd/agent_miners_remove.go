/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/filecoin-project/go-address"
	"github.com/glifio/cli/events"
	"github.com/glifio/go-pools/constants"
	"github.com/spf13/cobra"
)

var removePreview bool

// addCmd represents the add command
var rmCmd = &cobra.Command{
	Use:   "remove <miner address> <new owner address>",
	Short: "Remove a miner from your agent",
	Long: `Removes a specific miner from your Agent by assigning its owner to "new owner address". 
	The new owner address must be a filecoin address, not a delegated address.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if removePreview {
			previewAction(cmd, args, constants.MethodRemoveMiner)
			return
		}

		agentAddr, ownerWallet, ownerAccount, ownerPassphrase, proposer, approver, requesterKey, err := commonSetupOwnerCall()
		if err != nil {
			logFatal(err)
		}

		minerAddr, err := address.NewFromString(args[0])
		if err != nil {
			logFatal(err)
		}

		newMinerOwnerAddr, err := address.NewFromString(args[1])
		if err != nil {
			logFatal(err)
		}
		// IMPORTANT: an ethereum address can not be an owner of a miner, this must be a filecoin address owner
		if newMinerOwnerAddr.Protocol() == address.Delegated {
			logFatal("New miner owner address must be a filecoin address, not a delegated address")
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		removeevt := journal.RegisterEventType("agent", "removeminer")
		evt := &events.AgentMinerRemove{
			AgentID:  agentAddr.String(),
			MinerID:  minerAddr.String(),
			NewOwner: newMinerOwnerAddr.String(),
		}
		defer journal.Close()
		defer journal.RecordEvent(removeevt, func() interface{} { return evt })

		fmt.Printf("Removing miner %s from agent %s by changing its owner address to %s\n", minerAddr, agentAddr, newMinerOwnerAddr)

		txHash, _, err := PoolsSDK.Act().AgentRemoveMiner(cmd.Context(), agentAddr, minerAddr, newMinerOwnerAddr, ownerWallet, ownerAccount, ownerPassphrase, proposer, approver, requesterKey)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}
		evt.Tx = txHash.String()

		// transaction landed on chain or errored
		_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), txHash)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}

		s.Stop()

		fmt.Printf("Successfully proposed an ownership change to miner %s, passing %s as the new owner\n", minerAddr, newMinerOwnerAddr)
	},
}

func init() {
	minersCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVar(&removePreview, "preview", false, "preview the financial outcome of a remove miner action")
}

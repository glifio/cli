/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/briandowns/spinner"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/glifio/cli/events"
	"github.com/glifio/go-pools/constants"
	"github.com/spf13/cobra"
)

var addPreview bool

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <miner address>",
	Short: "Add a miner id to the agent",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			logFatal(err)
		}
		defer closer()

		if addPreview {
			previewAction(cmd, args, constants.MethodAddMiner)
			return
		}
		agentAddr, ownerWallet, ownerAccount, ownerPassphrase, requesterKey, err := commonSetupOwnerCall()
		if err != nil {
			logFatal(err)
		}

		minerAddr, err := address.NewFromString(args[0])
		if err != nil {
			logFatal(err)
		}

		mi, err := lapi.StateMinerInfo(cmd.Context(), minerAddr, types.EmptyTSK)
		if err != nil {
			logFatal(err)
		}

		if mi.Owner.String() != mi.Beneficiary.String() {
			logFatalf("Miner %s has a different owner (%s) and beneficiary (%s). Please reset the miner's beneficiary to match the owner before adding", minerAddr, mi.Owner, mi.Beneficiary)
		}

		log.Printf("Adding miner %s to agent %s", minerAddr, agentAddr)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		addminerevt := journal.RegisterEventType("agent", "addminer")
		evt := &events.AgentAddMiner{
			AgentID: agentAddr.String(),
			MinerID: minerAddr.String(),
		}
		defer journal.Close()
		defer journal.RecordEvent(addminerevt, func() interface{} { return evt })

		txHash, _, err := PoolsSDK.Act().AgentAddMiner(
			cmd.Context(),
			agentAddr,
			minerAddr,
			ownerWallet,
			ownerAccount,
			ownerPassphrase,
			requesterKey,
		)
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

		fmt.Printf("Successfully added miner %s to agent %s\n", minerAddr, agentAddr)
	},
}

func init() {
	minersCmd.AddCommand(addCmd)
	addCmd.Flags().BoolVar(&addPreview, "preview", false, "preview the financial outcome of an add miner action")
}

/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/glifio/cli/events"
	"github.com/spf13/cobra"
)

// pull represents the pull command
var pullFundsCmd = &cobra.Command{
	Use:   "pull-funds <miner address> <amount>",
	Short: "Pull FIL from a miner into your Glif Agent",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		from := cmd.Flag("from").Value.String()
		agentAddr, senderWallet, senderAccount, senderPassphrase, proposer, approver, requesterKey, err := commonOwnerOrOperatorSetup(ctx, from)
		if err != nil {
			logFatal(err)
		}

		minerAddr, err := ToMinerID(ctx, args[0])
		if err != nil {
			logFatal(err)
		}

		amount, err := parseFILAmount(args[1])
		if err != nil {
			logFatal(err)
		}

		fmt.Printf("Pulling %s FIL from %s", amount.String(), minerAddr.String())

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		pullevt := journal.RegisterEventType("agent", "pull")
		evt := &events.AgentMinerPull{
			AgentID: agentAddr.String(),
			MinerID: minerAddr.String(),
			Amount:  amount.String(),
		}
		defer journal.Close()
		defer journal.RecordEvent(pullevt, func() interface{} { return evt })

		txHash, _, err := PoolsSDK.Act().AgentPullFunds(ctx, agentAddr, amount, minerAddr, senderWallet, senderAccount, senderPassphrase, proposer, approver, requesterKey)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}
		evt.Tx = txHash.String()

		// transaction landed on chain or errored
		_, err = PoolsSDK.Query().StateWaitReceipt(ctx, txHash)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}

		s.Stop()

		fmt.Printf("Successfully pulled funds up from miner %s\n", minerAddr)
	},
}

func init() {
	minersCmd.AddCommand(pullFundsCmd)
	pullFundsCmd.Flags().String("from", "", "address of the owner or operator of the agent")
}

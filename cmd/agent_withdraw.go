/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/ethereum/go-ethereum/common"
	"github.com/glifio/cli/events"
	"github.com/glifio/go-pools/constants"
	denoms "github.com/glifio/go-pools/util"
	"github.com/spf13/cobra"
)

var withdrawPreview bool

// borrowCmd represents the borrow command
var withdrawCmd = &cobra.Command{
	Use:   "withdraw <amount> [flags]",
	Short: "Withdraw FIL from your Agent.",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if withdrawPreview {
			previewAction(cmd, args, constants.MethodWithdraw)
			return
		}

		agentAddr, ownerKey, requesterKey, err := commonSetupOwnerCall()
		if err != nil {
			logFatal(err)
		}

		var receiver common.Address
		if cmd.Flag("to") != nil && cmd.Flag("to").Changed {
			receiver, err = MustBeEVMAddr(cmd.Flag("to").Value.String())
		} else {
			receiver, err = PoolsSDK.Query().AgentOwner(cmd.Context(), agentAddr)
		}
		if err != nil {
			logFatal(err)
		}

		if !common.IsHexAddress(receiver.String()) {
			logFatal("Invalid withdraw address")
		}

		amount, err := parseFILAmount(args[0])
		if err != nil {
			logFatal(err)
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		withdrawevt := journal.RegisterEventType("agent", "withdraw")
		evt := &events.AgentWithdraw{
			AgentID: agentAddr.String(),
			Amount:  amount.String(),
			To:      receiver.String(),
		}
		defer journal.Close()
		defer journal.RecordEvent(withdrawevt, func() interface{} { return evt })

		fmt.Printf("Withdrawing %v FIL from your Agent to: %s", denoms.ToFIL(amount), receiver.String())

		tx, err := PoolsSDK.Act().AgentWithdraw(cmd.Context(), agentAddr, receiver, amount, ownerKey, requesterKey)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}
		evt.Tx = tx.Hash().String()

		_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), tx.Hash())
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}

		s.Stop()

		fmt.Printf("Successfully withdrew %s FIL\n", args[0])
	},
}

func init() {
	agentCmd.AddCommand(withdrawCmd)
	withdrawCmd.Flags().BoolVar(&withdrawPreview, "preview", false, "preview the financial outcome of a withdraw action")
	withdrawCmd.Flags().String("to", "", "Where to withdraw the funds to (note only 0x addresses are supported at the moment)")
}

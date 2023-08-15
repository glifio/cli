/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/glifio/cli/events"
	"github.com/glifio/go-pools/constants"
	"github.com/spf13/cobra"
)

var withdrawPreview bool

// borrowCmd represents the borrow command
var withdrawCmd = &cobra.Command{
	Use:   "withdraw <amount> <receiver>",
	Short: "Withdraw FIL from your Agent.",
	Long:  "",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if withdrawPreview {
			previewAction(cmd, args, constants.MethodWithdraw)
			return
		}

		agentAddr, ownerWallet, ownerAccount, ownerPassphrase, requesterKey, err := commonSetupOwnerCall()
		if err != nil {
			logFatal(err)
		}

		receiver, err := ParseAddressToEVM(cmd.Context(), args[1])
		if err != nil {
			logFatal(err)
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

		fmt.Printf("Withdrawing %s FIL from your Agent", args[0])

		txHash, _, err := PoolsSDK.Act().AgentWithdraw(cmd.Context(), agentAddr, receiver, amount, ownerWallet, ownerAccount, ownerPassphrase, requesterKey)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}
		evt.Tx = txHash.String()

		_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), txHash)
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
}

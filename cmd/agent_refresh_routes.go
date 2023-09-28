package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var refreshRoutesCmd = &cobra.Command{
	Use:   "refresh-routes",
	Short: "Update cached routes on your Agent",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		from := cmd.Flag("from").Value.String()
		agentAddr, senderWallet, senderAccount, senderPassphrase, proposer, approver, _, err := commonOwnerOrOperatorSetup(ctx, from)
		if err != nil {
			logFatal(err)
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		txHash, _, err := PoolsSDK.Act().AgentRefreshRoutes(ctx, agentAddr, senderWallet, senderAccount, senderPassphrase, proposer, approver)
		if err != nil {
			logFatalf("Failed to refresh routes %s", err)
		}

		_, err = PoolsSDK.Query().StateWaitReceipt(ctx, txHash)
		if err != nil {
			logFatalf("Failed to refresh routes %s", err)
		}

		s.Stop()

		fmt.Printf("Routes refreshed!\n")
	},
}

func init() {
	agentCmd.AddCommand(refreshRoutesCmd)
	refreshRoutesCmd.Flags().String("from", "", "address to send the transaction from")
}

package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var iFILApproveCmd = &cobra.Command{
	Use:   "approve <spender> <allowance>",
	Short: "Approve another address to spend your iFIL",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		_, pk, _, err := commonOwnerOrOperatorSetup(cmd)
		if err != nil {
			logFatal(err)
		}

		strAddr := args[0]
		strAmt := args[1]
		fmt.Printf("Approving %s to spend %s of your iFIL balance...\n", strAddr, strAmt)

		addr, err := ParseAddressToEVM(cmd.Context(), strAddr)
		if err != nil {
			logFatalf("Failed to parse address %s", err)
		}

		amount, err := parseFILAmount(strAmt)
		if err != nil {
			logFatalf("Failed to parse amount %s", err)
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		tx, err := PoolsSDK.Act().IFILApprove(cmd.Context(), addr, amount, pk)
		if err != nil {
			logFatalf("Failed to approve iFIL %s", err)
		}

		_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), tx.Hash())
		if err != nil {
			logFatalf("Failed to approve iFIL %s", err)
		}

		s.Stop()

		fmt.Printf("iFIL approved!\n")
	},
}

func init() {
	iFILCmd.AddCommand(iFILApproveCmd)
	iFILApproveCmd.Flags().String("from", "", "address of the owner or operator of the agent")
}

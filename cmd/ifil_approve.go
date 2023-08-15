package cmd

import (
	"fmt"
	"math/big"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var iFILApproveCmd = &cobra.Command{
	Use:   "approve <spender> <allowance>",
	Short: "Approve another address to spend your iFIL",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		from := cmd.Flag("from").Value.String()
		_, senderWallet, senderAccount, senderPassphrase, _, err := commonOwnerOrOperatorSetup(ctx, from)
		if err != nil {
			logFatal(err)
		}

		strAddr := args[0]
		strAmt := args[1]
		fmt.Printf("Approving %s to spend %s of your iFIL balance...", strAddr, strAmt)

		addr, err := ParseAddressToEVM(ctx, strAddr)
		if err != nil {
			logFatalf("Failed to parse address %s", err)
		}

		amt := big.NewInt(0)
		amt, ok := amt.SetString(strAmt, 10)
		if !ok {
			logFatalf("Failed to parse amount %s", err)
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		txHash, _, err := PoolsSDK.Act().IFILApprove(ctx, addr, amt, senderWallet, senderAccount, senderPassphrase)
		if err != nil {
			logFatalf("Failed to approve iFIL %s", err)
		}

		_, err = PoolsSDK.Query().StateWaitReceipt(ctx, txHash)
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

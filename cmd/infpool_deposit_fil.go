/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var depositFILCmd = &cobra.Command{
	Use:   "deposit-fil [amount]",
	Short: "Deposit FIL into the Infinity Pool",
	Args:  cobra.ExactArgs(1),
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		from := cmd.Flag("from").Value.String()
		_, senderWallet, senderAccount, senderPassphrase, proposer, approver, _, err := commonOwnerOrOperatorSetup(ctx, from)
		if err != nil {
			logFatal(err)
		}

		receiver := senderAccount.EthAccount.Address

		amount, err := parseFILAmount(args[0])
		if err != nil {
			logFatal(err)
		}

		fmt.Printf("Depositing %s FIL into the Infinity Pool", amount.String())

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		txHash, _, err := PoolsSDK.Act().InfPoolDepositFIL(ctx, receiver, amount, senderWallet, senderAccount, senderPassphrase, proposer, approver)
		if err != nil {
			logFatal(err)
		}

		// transaction landed on chain or errored
		receipt, err := PoolsSDK.Query().StateWaitReceipt(ctx, txHash)
		if err != nil {
			logFatal(err)
		}

		if receipt == nil {
			logFatal("Failed to get receipt")
		}

		if receipt.Status == 0 {
			logFatal("Transaction failed")
		}

		s.Stop()

		fmt.Printf("Successfully deposited funds into the Infinity Pool\n")
	},
}

func init() {
	infinitypoolCmd.AddCommand(depositFILCmd)
	depositFILCmd.Flags().String("from", "", "address of the owner or operator of the agent")
}

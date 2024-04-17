/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/glifio/cli/events"
	"github.com/glifio/go-pools/abigen"
	"github.com/glifio/go-pools/constants"
	"github.com/glifio/go-pools/deploy"
	denoms "github.com/glifio/go-pools/util"
	"github.com/jimpick/go-ethereum/common"
	"github.com/spf13/cobra"
)

var forwardFIL = &cobra.Command{
	Use:   "forward-fil <from> <to> <amount>",
	Short: "Transfers balances from an account to another address through the FilForwarder smart contract",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		from := args[0]
		auth, senderAccount, err := commonGenericAccountSetup(ctx, from)
		if err != nil {
			logFatal(err)
		}

		ethClient, err := PoolsSDK.Extern().ConnectEthClient()
		if err != nil {
			logFatal(err)
		}
		defer ethClient.Close()

		toStr := args[1]

		to, err := AddressOrAccountNameToNative(cmd.Context(), toStr)
		if err != nil {
			logFatal(err)
		}

		value, err := parseFILAmount(args[2])
		if err != nil {
			logFatal(err)
		}

		if value.Cmp(common.Big0) < 1 {
			logFatal(errors.New("Value must be greater than 0"))
		}

		if toStr == to.String() {
			fmt.Printf("Forwarding %0.09f FIL to %s\n", denoms.ToFIL(value), to.String())
		} else {
			fmt.Printf("Forwarding %0.09f FIL to %s (converted to %s)\n", denoms.ToFIL(value), toStr, to.String())
		}
		fmt.Println("(Note that on block explorers, the transaction's `to` address will be the FilForwarder smart contract address, which will forward the funds to the receiver address)")

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		forwardFILevt := journal.RegisterEventType("wallet", "forwardFIL")
		evt := &events.WalletFILForward{
			From:   args[0],
			To:     args[1],
			Amount: args[2],
		}
		defer journal.Close()
		defer journal.RecordEvent(forwardFILevt, func() interface{} { return evt })

		nonce, err := PoolsSDK.Query().ChainGetNonce(cmd.Context(), senderAccount.Address)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}

		chainID := PoolsSDK.Query().ChainID()

		var filForwardAddr common.Address
		switch chainID.Int64() {
		case constants.MainnetChainID:
			filForwardAddr = deploy.FilForwarder
		case constants.CalibnetChainID:
			filForwardAddr = deploy.TFilForwarder
		case constants.LocalnetChainID:
			filForwardAddr = common.HexToAddress(os.Getenv("GLIF_FIL_FORWARDER"))
		default:
			err = errors.New("unsupported chain id for forward-fil command")
			evt.Error = err.Error()
			logFatal(err)
		}

		// get the FilForwarder contract address
		filf, err := abigen.NewFilForwarderTransactor(filForwardAddr, ethClient)
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}

		auth.Nonce = nonce
		auth.Value = value

		tx, err := filf.Forward(auth, to.Bytes())
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}
		evt.Tx = tx.Hash().String()
		s.Stop()

		fmt.Printf("Forward FIL transaction sent: %s\n", tx.Hash().Hex())
		fmt.Println("Waiting for transaction to confirm...")

		s.Start()

		_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), tx.Hash())
		if err != nil {
			evt.Error = err.Error()
			logFatal(err)
		}

		s.Stop()

		fmt.Println("Success!")
	},
}

func init() {
	walletCmd.AddCommand(forwardFIL)
}

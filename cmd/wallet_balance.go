/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/briandowns/spinner"
	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-address"
	"github.com/glifio/cli/util"
	denoms "github.com/glifio/go-pools/util"
	"github.com/spf13/cobra"
)

// parallelizes balance fetching across accounts
func getBalances(
	ctx context.Context,
	owner address.Address,
	operator address.Address,
	request address.Address,
) (
	ownerBal *big.Float,
	operatorBal *big.Float,
	requesterBal *big.Float,
	err error,
) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		logFatalf("Failed to instantiate eth client %s", err)
	}
	defer closer()

	type balance struct {
		bal *big.Float
		key util.KeyType
	}

	balCh := make(chan balance)
	errCh := make(chan error)

	getBalAsync := func(key util.KeyType, addr address.Address) {
		bal, err := lapi.WalletBalance(ctx, addr)
		if err != nil {
			errCh <- err
			return
		}
		if bal.Int == nil {
			err = fmt.Errorf("failed to get %s balance", key)
			errCh <- err
			return
		}
		balDecimal := denoms.ToFIL(bal.Int)
		balCh <- balance{bal: balDecimal, key: key}
	}

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Start()
	defer s.Stop()

	go getBalAsync(util.OwnerKey, owner)
	go getBalAsync(util.OperatorKey, operator)
	go getBalAsync(util.RequestKey, request)

	s.Stop()

	for i := 0; i < 3; i++ {
		select {
		case bal := <-balCh:
			switch bal.key {
			case util.OwnerKey:
				ownerBal = bal.bal
			case util.OperatorKey:
				operatorBal = bal.bal
			case util.RequestKey:
				requesterBal = bal.bal
			}
		case err := <-errCh:
			return nil, nil, nil, err
		}
	}

	return ownerBal, operatorBal, requesterBal, nil
}

func logBal(key util.KeyType, bal *big.Float, fevmAddr address.Address, evmAddr common.Address) {
	if bal == nil {
		log.Printf("Failed to get %s balance", key)
		return
	}
	bf64, _ := bal.Float64()
	log.Printf("%s balance: %.02f FIL", key, bf64)
}

// newCmd represents the new command
var balCmd = &cobra.Command{
	Use:   "balance",
	Short: "Gets the balances associated with your accounts",
	Run: func(cmd *cobra.Command, args []string) {
		as := util.AccountsStore()
		ownerEvm, ownerFevm, err := as.GetAddrs(string(util.OwnerKey))
		if err != nil {
			logFatal(err)
		}

		operatorEvm, operatorFevm, err := as.GetAddrs(string(util.OperatorKey))
		if err != nil {
			logFatal(err)
		}

		requestEvm, requestFevm, err := as.GetAddrs(string(util.RequestKey))
		if err != nil {
			logFatal(err)
		}

		ownerBal, operatorBal, requesterBal, err := getBalances(
			cmd.Context(),
			ownerFevm,
			operatorFevm,
			requestFevm,
		)

		logBal(util.OwnerKey, ownerBal, ownerFevm, ownerEvm)
		logBal(util.OperatorKey, operatorBal, operatorFevm, operatorEvm)
		logBal(util.RequestKey, requesterBal, requestFevm, requestEvm)
	},
}

func init() {
	walletCmd.AddCommand(balCmd)
}

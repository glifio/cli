/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/briandowns/spinner"
	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-address"
	"github.com/glif-confidential/cli/util"
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
		log.Fatalf("Failed to instantiate eth client %s", err)
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
		}
		balDecimal := denoms.ToFIL(bal.Int)
		balCh <- balance{bal: balDecimal, key: key}
	}

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Start()

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
	bf64, _ := bal.Float64()
	log.Printf("%s balance: %.02f FIL", key, bf64)
}

// newCmd represents the new command
var balCmd = &cobra.Command{
	Use:   "balance",
	Short: "Gets the balances associated with your owner and operator keys",
	Run: func(cmd *cobra.Command, args []string) {
		ks := util.KeyStore()
		ownerEvm, ownerFevm, err := ks.GetAddrs(util.OwnerKey)
		if err != nil {
			log.Fatal(err)
		}

		operatorEvm, operatorFevm, err := ks.GetAddrs(util.OperatorKey)
		if err != nil {
			log.Fatal(err)
		}

		requestEvm, requestFevm, err := ks.GetAddrs(util.RequestKey)
		if err != nil {
			log.Fatal(err)
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

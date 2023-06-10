/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"math/big"
	"time"

	"github.com/briandowns/spinner"
	"github.com/glifio/cli/events"
	"github.com/spf13/cobra"
)

var payCmd = &cobra.Command{
	Use: "pay",
}

func init() {
	agentCmd.AddCommand(payCmd)
}

func pay(cmd *cobra.Command, args []string, paymentType string) (*big.Int, error) {
	agentAddr, senderKey, requesterKey, err := commonOwnerOrOperatorSetup(cmd)
	if err != nil {
		return nil, err
	}

	var payAmt *big.Int

	switch paymentType {
	case "principal":
		amount, err := parseFILAmount(args[0])
		if err != nil {
			return nil, err
		}

		amountOwed, _, err := PoolsSDK.Query().AgentOwes(cmd.Context(), agentAddr)
		if err != nil {
			return nil, err
		}

		payAmt = new(big.Int).Add(amount, amountOwed)
	case "to-current":
		amountOwed, _, err := PoolsSDK.Query().AgentOwes(cmd.Context(), agentAddr)
		if err != nil {
			return nil, err
		}

		payAmt = amountOwed
	case "custom":
		amount, err := parseFILAmount(args[0])
		if err != nil {
			return nil, err
		}

		payAmt = amount
	default:
		return nil, fmt.Errorf("invalid payment type: %s", paymentType)
	}

	poolName := cmd.Flag("pool-name").Value.String()

	poolID, err := parsePoolType(poolName)
	if err != nil {
		return nil, err
	}

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Start()
	defer s.Stop()

	payevt := journal.RegisterEventType("agent", "pay")
	evt := &events.AgentPay{
		AgentID: agentAddr.String(),
		PoolID:  poolID.String(),
		Amount:  payAmt.String(),
		PayType: paymentType,
	}
	defer journal.Close()
	defer journal.RecordEvent(payevt, func() interface{} { return evt })

	tx, err := PoolsSDK.Act().AgentPay(cmd.Context(), agentAddr, poolID, payAmt, senderKey, requesterKey)
	if err != nil {
		evt.Error = err.Error()
		return nil, err
	}
	evt.Tx = tx.Hash().String()

	// transaction landed on chain or errored
	_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), tx.Hash())
	if err != nil {
		evt.Error = err.Error()
		return nil, err
	}

	s.Stop()

	return payAmt, nil
}

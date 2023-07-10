package cmd

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/glifio/cli/events"
	"github.com/glifio/cli/util"
	"github.com/glifio/go-pools/abigen"
	"github.com/glifio/go-pools/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var agentAutopilotCmd = &cobra.Command{
	Use:   "autopilot",
	Short: "Background service that automatically repays FIL to pools",
	Long:  `Background service that automatically repays FIL to pools.`,
	Run: func(cmd *cobra.Command, args []string) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
			}
		}()

		defer journal.Close()

		if cmd.Flag("logfile") != nil && cmd.Flag("logfile").Changed {
			file, err := os.OpenFile(cmd.Flag("logfile").Value.String(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
			if err != nil {
				log.Fatal(err)
			}
			log.SetOutput(file)
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		ctx := cmd.Context()

		log.Println("Starting autopilot...")

		log.Println("Lotus Daemon: ", viper.GetString("daemon.rpc-url"))
		for {
			select {
			case <-sigs:
				log.Println("Shutting down...")
				Exit(0)
			default:
				log.Println("Checking for payments...")

				// CONFIG options
				// each loop retrieve config values aka hot-reload
				paymentType, err := ParsePaymentType(viper.GetString("autopilot.payment-type"))
				if err != nil {
					log.Println(err)
					continue
				}
				log.Println("Payment type: ", paymentType)
				payargs := []string{}
				if paymentType == Principal || paymentType == Custom {
					amount := viper.GetInt64("autopilot.amount")
					payargs = append(payargs, fmt.Sprintf("%d", amount))
				}

				pullFundsEnabled := viper.GetBool("autopilot.pullfunds.enabled")
				pullFundsFactor := viper.GetInt("autopilot.pullfunds.pull-amount-factor")

				//TODO: maybe change frequency to max debt or max epoch difference
				frequency := viper.GetFloat64("autopilot.frequency")

				log.Println("frequency: ", frequency)

				// goto can't jump over variable declarations
				var chainHeadHeight *big.Int
				var account abigen.Account

				// GATHER variables
				agent, err := getAgentAddress(cmd)
				if err != nil {
					log.Println(err)
					goto SLEEP
				}

				account, err = PoolsSDK.Query().InfPoolGetAccount(ctx, agent, nil)
				if err != nil {
					log.Println(err)
					goto SLEEP
				}
				if account == (abigen.Account{}) {
					log.Println("failed to get infinity pool account, check evm api provider status")
					goto SLEEP
				}

				chainHeadHeight, err = PoolsSDK.Query().ChainHeight(cmd.Context())
				if err != nil {
					log.Println(err)
					goto SLEEP
				}
				if chainHeadHeight == nil {
					log.Println("failed to get chainheight, check lotus api provider status")
					goto SLEEP
				}

				// check if payment is due
				// if so, make payment
				if paymentDue(frequency, chainHeadHeight.Uint64(), account.EpochsPaid) {
					if pullFundsEnabled {
						payAmt, err := payAmount(cmd, payargs, paymentType)
						if err != nil {
							log.Println(err)
							goto SLEEP
						}

						if pull, err := needToPullFunds(cmd, payAmt); pull && err == nil {
							factoredPullAmt := big.NewInt(0).Mul(payAmt, big.NewInt(int64(pullFundsFactor)))
							miner, ok, err := chooseMiner(cmd, factoredPullAmt)
							if err != nil {
								log.Println(err)
								goto SLEEP
							}
							if !ok {
								log.Println(fmt.Errorf("no miner available to pull full amount of required funds from, please manually transfer FIL from miners to agent"))
								goto SLEEP
							}

							err = pullFundsFromMiner(cmd, miner, factoredPullAmt)
							if err != nil {
								log.Println(err)
								goto SLEEP
							}
						}

					}
					_, err = pay(cmd, payargs, paymentType, true)
					if err != nil {
						log.Println(err)
					}

				}
			SLEEP:
				select {
				case <-time.After(30 * time.Minute):
					continue
				case <-sigs:
					log.Println("Shutting down...")
					Exit(0)
				}
			}
		}
	},
}

func paymentDue(frequency float64, chainHeadHeight uint64, epochsPaid *big.Int) bool {
	epochFreq := big.NewFloat(float64(frequency * constants.EpochsInDay))

	dueEpoch := new(big.Int).Sub(new(big.Int).SetUint64(chainHeadHeight), epochsPaid)

	epochFreqInt64, _ := epochFreq.Int64()
	epochFreqInt := big.NewInt(epochFreqInt64)

	return dueEpoch.Cmp(epochFreqInt) >= 0
}

// needToPullFunds returns whether the payAmt is larger than the current
// balance of the Operator wallet.
func needToPullFunds(cmd *cobra.Command, payAmt *big.Int) (bool, error) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return false, fmt.Errorf("Failed to instantiate eth client %s", err)
	}
	defer closer()

	ks := util.KeyStore()
	_, operatorFevm, err := ks.GetAddrs(util.OperatorKey)
	if err != nil {
		return false, err
	}

	bal, err := lapi.WalletBalance(cmd.Context(), operatorFevm)
	if err != nil {
		return false, err
	}

	return payAmt.Cmp(bal.Int) > 0, nil
}

func pullFundsFromMiner(cmd *cobra.Command, miner address.Address, amount *big.Int) error {
	agentAddr, senderKey, requesterKey, err := commonOwnerOrOperatorSetup(cmd)
	if err != nil {
		return err
	}
	pullevt := journal.RegisterEventType("agent", "pull")
	evt := &events.AgentMinerPull{
		AgentID: agentAddr.String(),
		MinerID: miner.String(),
		Amount:  amount.String(),
	}
	defer journal.RecordEvent(pullevt, func() interface{} { return evt })

	tx, err := PoolsSDK.Act().AgentPullFunds(cmd.Context(), agentAddr, amount, miner, senderKey, requesterKey)
	if err != nil {
		evt.Error = err.Error()
		return err
	}
	evt.Tx = tx.Hash().String()

	// transaction landed on chain or errored
	_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), tx.Hash())
	if err != nil {
		evt.Error = err.Error()
		return err
	}
	return nil
}

// chooseMiner returns the 'richest' miner associated with the agent and whether that miner
// has more funds than the requiredFunds parameter.
func chooseMiner(cmd *cobra.Command, requiredFunds *big.Int) (address.Address, bool, error) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return address.Address{}, false, fmt.Errorf("Failed to instantiate eth client %s", err)
	}
	defer closer()
	agentAddr, _, _, err := commonOwnerOrOperatorSetup(cmd)
	if err != nil {
		return address.Address{}, false, err
	}

	miners, err := PoolsSDK.Query().AgentMiners(cmd.Context(), agentAddr)
	if err != nil {
		return address.Address{}, false, err
	}

	var chosen address.Address
	var bal *big.Int
	for i, m := range miners {
		mbal, err := lapi.StateMinerAvailableBalance(cmd.Context(), m, types.EmptyTSK)
		if err != nil {
			return address.Address{}, false, err
		}
		if i == 0 {
			chosen = m
			bal = mbal.Int
			continue
		}
		if mbal.Int.Cmp(bal) > 0 {
			chosen = m
			bal = mbal.Int
		}
	}
	return chosen, bal.Cmp(requiredFunds) > 0, nil
}

func init() {
	agentCmd.AddCommand(agentAutopilotCmd)
	agentAutopilotCmd.Flags().String("agent-addr", "", "Agent address")
	agentAutopilotCmd.Flags().String("pool-name", "infinity-pool", "name of the pool to make a payment")
	agentAutopilotCmd.Flags().String("from", "", "address to send the transaction from")
	agentAutopilotCmd.Flags().String("logfile", "", "Logfile path, if empty autopilot logs to stderr")
}

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
	"github.com/glifio/cli/journal/fsjournal"
	"github.com/glifio/go-pools/abigen"
	"github.com/glifio/go-pools/constants"
	"github.com/glifio/go-pools/util"
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
			var err error
			if journal, err = fsjournal.OpenFSJournal(cfgDir, nil); err != nil {
				logFatal(err)
			}
			defer journal.Close()

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
				log.Println("pullfunds: ", pullFundsEnabled)
				log.Println("pullfunds-factor: ", pullFundsFactor)

				//TODO: maybe change frequency to max debt or max epoch difference
				frequency := viper.GetFloat64("autopilot.frequency")

				log.Println("frequency (days): ", frequency)

				// goto can't jump over variable declarations
				var chainHeadHeight *big.Int
				var account abigen.Account
				var pullFundsMiner address.Address

				agent, err := getAgentAddressWithFlags(cmd)
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
				if paymentDue(frequency, chainHeadHeight, account.EpochsPaid) {
					if pullFundsEnabled {
						pullFundsMiner, err = ToMinerID(cmd.Context(), viper.GetString("autopilot.pullfunds.miner"))
						if err != nil {
							log.Println(err)
							goto SLEEP
						}

						payAmt, err := payAmount(ctx, cmd, payargs, paymentType)
						if err != nil {
							log.Println(err)
							goto SLEEP
						}

						pull, err := needToPullFunds(cmd, payAmt)
						if err != nil {
							log.Println(err)
							goto SLEEP
						}

						if pull {
							factoredPullAmt := new(big.Int).Mul(payAmt, big.NewInt(int64(pullFundsFactor)))

							factoredPullAmtFIL, _ := util.ToFIL(factoredPullAmt).Float64()
							log.Printf("Pulling %0.08f (or max available) from miner %s", factoredPullAmtFIL, pullFundsMiner)
							err = pullFundsFromMiner(cmd, pullFundsMiner, factoredPullAmt)
							if err != nil {
								log.Println(err)
								goto SLEEP
							}
						}

					}

					log.Printf("Making payment: %v", payargs)
					_, err = pay(cmd, payargs, paymentType)
					if err != nil {
						log.Println(err)
					}

				}
			SLEEP:
				sleepTime := 30 * time.Minute
				if debugSetup {
					sleepTime = 30 * time.Second
				}
				select {
				case <-time.After(sleepTime):
					continue
				case <-sigs:
					log.Println("Shutting down...")
					Exit(0)
				}
			}
		}
	},
}

func paymentDue(frequency float64, chainHeadHeight, epochsPaid *big.Int) bool {
	epochFreq := big.NewFloat(float64(frequency * constants.EpochsInDay))

	epochsPassed := new(big.Int).Sub(chainHeadHeight, epochsPaid)

	epochFreqInt64, _ := epochFreq.Int64()
	epochFreqInt := big.NewInt(epochFreqInt64)

	return epochsPassed.Cmp(epochFreqInt) >= 0
}

// needToPullFunds returns whether the payAmt is larger than the agent
// liquid assets.
func needToPullFunds(cmd *cobra.Command, payAmt *big.Int) (bool, error) {
	agentAddr, err := getAgentAddressWithFlags(cmd)
	if err != nil {
		return false, err
	}

	assets, err := PoolsSDK.Query().AgentLiquidAssets(cmd.Context(), agentAddr, nil)
	if err != nil {
		return false, err
	}

	return payAmt.Cmp(assets) > 0, nil
}

func pullFundsFromMiner(cmd *cobra.Command, miner address.Address, amount *big.Int) error {
	ctx := cmd.Context()
	from := cmd.Flag("from").Value.String()
	agentAddr, senderWallet, senderAccount, senderPassphrase, requesterKey, err := commonOwnerOrOperatorSetup(ctx, from)
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

	txHash, _, err := PoolsSDK.Act().AgentPullFunds(cmd.Context(), agentAddr, amount, miner, senderWallet, senderAccount, senderPassphrase, requesterKey)
	if err != nil {
		evt.Error = err.Error()
		return err
	}
	evt.Tx = txHash.String()

	// transaction landed on chain or errored
	_, err = PoolsSDK.Query().StateWaitReceipt(cmd.Context(), txHash)
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
	agentAddr, err := getAgentAddressWithFlags(cmd)
	if err != nil {
		return address.Address{}, false, err
	}

	miners, err := PoolsSDK.Query().AgentMiners(cmd.Context(), agentAddr, nil)
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

var debugSetup bool

func init() {
	agentCmd.AddCommand(agentAutopilotCmd)
	agentAutopilotCmd.Flags().String("agent-addr", "", "Agent address")
	agentAutopilotCmd.Flags().String("pool-name", "infinity-pool", "name of the pool to make a payment")
	agentAutopilotCmd.Flags().String("from", "", "address to send the transaction from")
	agentAutopilotCmd.Flags().String("logfile", "", "Logfile path, if empty autopilot logs to stderr")
	agentAutopilotCmd.Flags().BoolVar(&debugSetup, "debug", false, "enable debug setup, i.e. 30 second sleep in main loop")
}

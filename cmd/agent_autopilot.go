package cmd

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/glifio/cli/journal/alerting"
	"github.com/glifio/cli/util"
	"github.com/glifio/go-pools/constants"
	denoms "github.com/glifio/go-pools/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var agentAutopilotCmd = &cobra.Command{
	Use:   "autopilot",
	Short: "Background service that automatically repays FIL to pools",
	Long:  `Background service that automatically repays FIL to pools.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting autopilot...")
		defer journal.Close()

		// setup alerting
		setupAlerts(cmd, []string{"agent:balance"})

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		ctx := cmd.Context()

		for {
			select {
			case <-sigs:
				log.Println("Shutting down...")
				Exit(0)
			default:
				checkAlerts(cmd)
				log.Println("Checking for payments...")
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

				//TODO: maybe change frequency to max debt or max epoch difference
				frequency := viper.GetFloat64("autopilot.frequency")

				agent, err := getAgentAddress(cmd)
				if err != nil {
					log.Println(err)
				}

				account, err := PoolsSDK.Query().InfPoolGetAccount(ctx, agent)
				if err != nil {
					log.Println(err)
				}

				chainHeadHeight, err := PoolsSDK.Query().ChainHeight(cmd.Context())
				if err != nil {
					log.Println(err)
				}

				// calculate epoch frequency
				epochFreq := big.NewFloat(float64(frequency * constants.EpochsInDay))

				dueEpoch := new(big.Int).Sub(new(big.Int).SetUint64(chainHeadHeight.Uint64()), account.EpochsPaid)

				epochFreqInt64, _ := epochFreq.Int64()
				epochFreqInt := big.NewInt(epochFreqInt64)

				// check if payment is due
				// if so, make payment
				if dueEpoch.Cmp(epochFreqInt) >= 0 {
					switch paymentType {
					case Principal:
						_, err := pay(cmd, payargs, paymentType, true)
						if err != nil {
							log.Println(err)
						}

					case ToCurrent:
						_, err := pay(cmd, payargs, paymentType, true)
						if err != nil {
							log.Println(err)
						}

					case Custom:
						_, err := pay(cmd, payargs, paymentType, true)
						if err != nil {
							log.Println(err)
						}

					default:
						log.Println("Invalid payment type")
					}
				}

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

var daemonAlerts []alerting.AlertType

func setupAlerts(cmd *cobra.Command, alrts []string) {
	// setup alerts
	for _, alt := range alrts {
		system, subsystem := splitAlertToStrings(alt)
		at := alerts.AddAlertType(system, subsystem)
		daemonAlerts = append(daemonAlerts, at)
	}
}

func splitAlertToStrings(alert string) (string, string) {
	// split colon delimited alert string into two strings
	// for example: "system:subsystem"
	// returns "system", "subsystem"

	strs := strings.Split(alert, ":")

	return strs[0], strs[1]
}

func checkAlerts(cmd *cobra.Command) {
	// check for alerts

	for _, alt := range daemonAlerts {
		switch {
		case alt.System == "agent" && alt.Subsystem == "balance":
			if lowBalance(cmd, 0.5) {
				alerts.Raise(alt, "Agent balance is low")
			}
		}
	}
}

func lowBalance(cmd *cobra.Command, threshold float64) bool {
	thres := big.NewFloat(threshold)

	ks := util.KeyStore()

	_, operatorFevm, err := ks.GetAddrs(util.OperatorKey)
	if err != nil {
		log.Printf("Failed to get operator address %s", err)
	}

	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		log.Printf("Failed to instantiate eth client %s", err)
	}
	defer closer()

	bal, err := lapi.WalletBalance(cmd.Context(), operatorFevm)
	if err != nil {
		log.Printf("Failed to get balance %s", err)
	}
	if bal.Int == nil {
		err = fmt.Errorf("failed to get balance")
		log.Println(err)
	}
	balDecimal := denoms.ToFIL(bal.Int)

	return balDecimal.Cmp(thres) < 0
}

func init() {
	agentCmd.AddCommand(agentAutopilotCmd)
	agentAutopilotCmd.Flags().String("agent-addr", "", "Agent address")
	agentAutopilotCmd.Flags().String("pool-name", "infinity-pool", "name of the pool to make a payment")
	agentAutopilotCmd.Flags().String("from", "", "address to send the transaction from")
}

package cmd

import (
	"context"
	"log"
	"time"

	"github.com/briandowns/spinner"
	"github.com/glifio/go-pools/rpc"
	"github.com/glifio/go-pools/util"

	"github.com/spf13/cobra"
)

var previewWithdrawMaxCmd = &cobra.Command{
	Use:   "withdraw-max",
	Short: "Get a quote for the maximum you can withdraw right now.",
	Run: func(cmd *cobra.Command, args []string) {
		agentAddr, err := getAgentAddressWithFlags(cmd)
		if err != nil {
			logFatal(err)
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		agentLiquidAssets, err := PoolsSDK.Query().AgentLiquidAssets(cmd.Context(), agentAddr, nil)
		if err != nil {
			logFatal(err)
		}

		closer, err := PoolsSDK.Extern().ConnectAdoClient(cmd.Context())
		if err != nil {
			logFatal(err)
		}
		defer closer()

		agentData, err := rpc.ADOClient.AgentData(context.Background(), agentAddr)
		if err != nil {
			logFatal(err)
		}

		maxWithdraw, err := PoolsSDK.Query().InfPoolAgentMaxWithdraw(cmd.Context(), agentAddr, agentData)
		if err != nil {
			logFatal(err)
		}

		s.Stop()

		log.Printf("Agent can withdraw up to %0.09f FIL\n", util.ToFIL(maxWithdraw))
		log.Printf("Agent has %0.09f FIL in liquid assets\n", util.ToFIL(agentLiquidAssets))
		log.Println("Borrowing funds may change this value.")
	},
}

func init() {
	previewCmd.AddCommand(previewWithdrawMaxCmd)
	previewWithdrawMaxCmd.Flags().String("agent-addr", "", "Agent address")
}

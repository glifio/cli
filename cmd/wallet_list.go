/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"log"

	"github.com/filecoin-project/lotus/api"
	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists the addresses associated with your owner, operator, and requester keys",
	Run: func(cmd *cobra.Command, args []string) {
		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			logFatalf("Failed to instantiate eth client %s", err)
		}
		defer closer()

		listAddresses(lapi)
	},
}

func listAddresses(lapi *api.FullNodeStruct) {
	as := util.AgentStore()
	ownerAddr, ownerFilAddr, err := as.GetAddrs(util.OwnerKey, lapi)
	if err != nil {
		logFatal(err)
	}
	operatorAddr, operatorDelAddr, err := as.GetAddrs(util.OperatorKey, nil)
	if err != nil {
		logFatal(err)
	}
	requestAddr, requestDelAddr, err := as.GetAddrs(util.RequestKey, nil)
	if err != nil {
		logFatal(err)
	}

	if util.IsZeroAddress(ownerAddr) {
		log.Printf("Owner address: [ Funds needed! ] (ETH), %s (FIL)\n", ownerFilAddr)

	} else {
		log.Printf("Owner address: %s (ETH), %s (FIL)\n", ownerAddr, ownerFilAddr)
	}
	log.Printf("Operator address: %s (ETH), %s (FIL)\n", operatorAddr, operatorDelAddr)
	log.Printf("Request key: %s (ETH), %s (FIL)\n", requestAddr, requestDelAddr)

}

func init() {
	walletCmd.AddCommand(listCmd)
}

/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"log"

	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists the addresses associated with your owner, operator, and requester keys",
	Run: func(cmd *cobra.Command, args []string) {
		ks := util.KeyStore()
		ownerEvm, ownerFevm, err := ks.GetAddrs(util.OwnerKey)
		if err != nil {
			logFatal(err)
		}

		operatorEvm, operatorFevm, err := ks.GetAddrs(util.OperatorKey)
		if err != nil {
			logFatal(err)
		}

		requestEvm, requestFevm, err := ks.GetAddrs(util.RequestKey)
		if err != nil {
			logFatal(err)
		}

		log.Printf("Owner address: %s (EVM), %s (FIL)", ownerEvm, ownerFevm)
		log.Printf("Operator address: %s (EVM), %s (FIL)", operatorEvm, operatorFevm)
		log.Printf("Requester address: %s (EVM), %s (FIL)", requestEvm, requestFevm)
	},
}

func init() {
	walletCmd.AddCommand(listCmd)
}

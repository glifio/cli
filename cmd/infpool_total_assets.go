/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var infpoolTotalAssetsCmd = &cobra.Command{
	Use:   "total-assets",
	Short: "Gets the details associated with an active account borrowing from the Infinity Pool",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Querying the Infinity Pool's total assets")

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		assets, err := PoolsSDK.Query().InfPoolTotalAssets(cmd.Context(), nil)
		if err != nil {
			logFatalf("Failed to get iFIL balance %s", err)
		}

		s.Stop()

		fmt.Printf("Infinity Pool total assets: %.04f FIL\n", assets)
	},
}

func init() {
	infinitypoolCmd.AddCommand(infpoolTotalAssetsCmd)
}

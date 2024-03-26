package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	denoms "github.com/glifio/go-pools/util"
	"github.com/spf13/cobra"
)

var iFILSupplyCmd = &cobra.Command{
	Use:   "supply",
	Short: "Get the iFIL supply",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Checking iFIL supply...")

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		supply, err := PoolsSDK.Query().IFILSupply(cmd.Context(), nil)
		if err != nil {
			logFatalf("Failed to get iFIL balance %s", err)
		}

		supplyFIL, _ := denoms.ToFIL(supply).Float64()

		s.Stop()

		fmt.Printf("%.09f iFIL\n", supplyFIL)
	},
}

func init() {
	iFILCmd.AddCommand(iFILSupplyCmd)
}

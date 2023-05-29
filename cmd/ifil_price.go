package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var iFILPriceCmd = &cobra.Command{
	Use:   "price",
	Short: "Get the iFIL price, denominated in FIL",
	Long:  "Get the iFIL price, denominated in FIL. The number returned is the amount of FIL that 1 iFIL is worth.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Checking iFIL prices...")

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		price, err := PoolsSDK.Query().IFILPrice(cmd.Context())
		if err != nil {
			log.Fatalf("Failed to get iFIL balance %s", err)
		}

		priceFIL, _ := price.Float64()

		s.Stop()

		fmt.Printf("1 iFIL is worth %.09f FIL", priceFIL)
	},
}

func init() {
	iFILCmd.AddCommand(iFILPriceCmd)
}

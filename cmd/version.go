package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the latest tag",
	Run: func(cmd *cobra.Command, args []string) {
		tagName, _, _, err := getLatestTag()
		if err != nil {
			logFatal(err)
		}
		fmt.Println(tagName)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

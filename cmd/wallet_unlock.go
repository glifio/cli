/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/glifio/glif/v2/util"
	"github.com/spf13/cobra"
)

// walletUnlockCmd represents the wallet unlock command
var walletUnlockCmd = &cobra.Command{
	Use:   "unlock [account name]",
	Args:  cobra.ExactArgs(1),
	Short: "Unlocks an account to check the password is correct",
	Run: func(cmd *cobra.Command, args []string) {
		ks := util.KeyStore()
		addr, _, err := util.AccountsStore().GetAddrs(args[0])
		if err != nil {
			logFatal(err)
		}

		account := accounts.Account{Address: addr}

		if !ks.HasAddress(addr) {
			logFatal("Address not found in keystore")
		}

		passphrase := ""
		prompt := &survey.Password{
			Message: "Passphrase",
		}
		survey.AskOne(prompt, &passphrase)

		if err := ks.Unlock(account, passphrase); err != nil {
			logFatal(err)
		}

		fmt.Println("Account unlocked: ", addr)
	},
}

func init() {
	walletCmd.AddCommand(walletUnlockCmd)
}

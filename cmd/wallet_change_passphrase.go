/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/glifio/cli/util"
	"github.com/jimpick/go-ethereum/accounts"
	"github.com/jimpick/go-ethereum/common"
	"github.com/spf13/cobra"
)

var changePassphraseCmd = &cobra.Command{
	Use:   "change-passphrase <account or address>",
	Short: "Change the passphrase for an encrypted key in the keystore",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var addr common.Address
		var err error
		if strings.HasPrefix(args[0], "0x") {
			addr = common.HexToAddress(args[0])
		} else {
			as := util.AccountsStore()
			addr, _, err = as.GetAddrs(args[0])
			if err != nil {
				var e *util.ErrKeyNotFound
				if errors.As(err, &e) {
					logFatal("Account not found in wallet")
				}
				logFatal(err)
			}
		}
		err = changePassphrase(addr)
		if err != nil {
			logFatal(err)
		}
	},
}

func init() {
	walletCmd.AddCommand(changePassphraseCmd)
}

func changePassphrase(addr common.Address) error {
	ks := util.KeyStore()

	account := accounts.Account{Address: addr}

	if !ks.HasAddress(addr) {
		logFatal("Address not found in keystore")
	}

	oldPassphrase := ""
	err := ks.Unlock(account, "")
	if err != nil {
		prompt := &survey.Password{
			Message: "Old passphrase",
		}
		survey.AskOne(prompt, &oldPassphrase)
	}

	newPassphrase, envSet := os.LookupEnv("GLIF_OWNER_PASSPHRASE")
	if !envSet {
		prompt := &survey.Password{
			Message: "New passphrase",
		}
		survey.AskOne(prompt, &newPassphrase)
		var confirmPassphrase string
		confirmPrompt := &survey.Password{
			Message: "Confirm passphrase",
		}
		survey.AskOne(confirmPrompt, &confirmPassphrase)
		if newPassphrase != confirmPassphrase {
			logFatal("Aborting. Passphrase confirmation did not match.")
		}
	}

	err = ks.Update(account, oldPassphrase, newPassphrase)
	if err != nil {
		return err
	}

	bs := util.BackupsStore()
	bs.Invalidate()

	fmt.Println("Passphrase successfully changed.")

	return nil
}

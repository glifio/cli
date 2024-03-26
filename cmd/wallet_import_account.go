/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// importAccountCmd represents the import-account command
var importAccountCmd = &cobra.Command{
	Use:   "import-account [account-name] [account-private-key]",
	Short: "Import a single private key account",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		as := util.AccountsStore()

		overWrite, err := cmd.Flags().GetBool("overwrite")
		if err != nil {
			logFatal(err)
		}

		protectWPassphrase, err := cmd.Flags().GetBool("add-passphrase")
		if err != nil {
			logFatal(err)
		}

		name := strings.ToLower(args[0])
		addrToOverwrite, err := as.Get(name)

		rename := fmt.Sprintf("%s-replaced-%s", name, time.Now().Format(time.RFC3339))

		var e *util.ErrKeyNotFound
		if !errors.As(err, &e) && !overWrite {
			logFatalf("Account %s already exists", name)
		} else if !errors.As(err, &e) {
			log.Printf("Warning: account '%s' already exists, renaming to '%s' and overriding with new '%s' key\n", name, rename, name)
		} else if overWrite {
			// here we dont actually have any keys to overwrite, so we set overWrite to false to avoid downstream issues
			overWrite = false
			log.Printf("Warning: no %s account to overwrite... importing %s\n", name, name)
		} else {
			log.Printf("Importing account: %s\n", name)
		}

		re := regexp.MustCompile(`^[tf][0-9]`)
		if strings.HasPrefix(name, "0x") || re.MatchString(name) {
			logFatalf("Invalid name")
		}

		pk := args[1]
		pkECDSA, err := crypto.HexToECDSA(pk)
		if err != nil {
			logFatalf("Invalid private key")
		}

		passphrase := ""
		if protectWPassphrase {
			passphrase, envSet := os.LookupEnv("GLIF_PASSPHRASE")
			if !envSet {
				prompt := &survey.Password{
					Message: "Please type a passphrase to encrypt your private key",
				}
				survey.AskOne(prompt, &passphrase)
				var confirmPassphrase string
				confirmPrompt := &survey.Password{
					Message: "Confirm passphrase",
				}
				survey.AskOne(confirmPrompt, &confirmPassphrase)
				if passphrase != confirmPassphrase {
					logFatal("Aborting. Passphrase confirmation did not match.")
				}
			}
		}

		ks := util.KeyStore()

		account, err := ks.ImportECDSA(pkECDSA, passphrase)
		if err != nil {
			logFatal(err)
		}

		// we rename the old named account to a new name so we dont lose a reference to this key
		if overWrite {
			as.Set(rename, addrToOverwrite)
		}

		as.Set(name, account.Address.String())

		if err := viper.WriteConfig(); err != nil {
			logFatal(err)
		}

		accountAddr, accountDelAddr, err := as.GetAddrs(name)
		if err != nil {
			logFatal(err)
		}

		bs := util.BackupsStore()
		bs.Invalidate()

		log.Printf("%s address: %s (ETH), %s (FIL)\n", name, accountAddr, accountDelAddr)
	},
}

func init() {
	walletCmd.AddCommand(importAccountCmd)
	importAccountCmd.Flags().Bool("overwrite", false, "overwrite an existing account with the same name")
	importAccountCmd.Flags().Bool("add-passphrase", false, "add a passphrase to encrypt the account")
}

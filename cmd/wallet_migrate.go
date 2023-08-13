/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"fmt"

	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrates keys from keys.toml to the new encrypted keystore",
	Run: func(cmd *cobra.Command, args []string) {
		err := checkWalletMigrated()
		if err == nil {
			fmt.Println("Wallet already migrated to encrypted keystore.")
			return
		}

		err = migrateLegacyKeys()
		if err != nil {
			logFatal(err)
		}
		fmt.Printf("Keys successfully migrated to the encrypted keystore!\n\n")

		fmt.Printf("Please set a new passphrase to encrypt the owner key:\n\n")

		as := util.AgentStore()
		ownerAddr, _, err := as.GetAddrs(util.OwnerKey, nil)
		if err != nil {
			logFatal(err)
		}
		err = changePassphrase(ownerAddr)
		if err != nil {
			logFatal(err)
		}

		fmt.Printf("\nFor increased security, please delete the legacy %s/keys.toml file with the cleartext private keys.\n", cfgDir)
		fmt.Println("The legacy keys.toml file is no longer needed, unless you are just testing and plan to downgrade.")
	},
}

func init() {
	walletCmd.AddCommand(migrateCmd)
}

func migrateLegacyKeys() error {
	keys := []util.KeyType{util.OwnerKey, util.OperatorKey, util.RequestKey}
	for _, key := range keys {
		if err := migrateLegacyKey(key); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyKey(key util.KeyType) error {
	ksLegacy := util.KeyStoreLegacy()
	ks := util.KeyStore()
	as := util.AgentStore()

	pkStr, err := ksLegacy.Get(string(key))
	if err != nil {
		return err
	}
	if pkStr == "" {
		return fmt.Errorf("migration failed, missing private key for %s in keys.toml", string(key))
	}
	pk, err := ksLegacy.GetPrivate(key)
	if err != nil {
		return err
	}
	account, err := ks.ImportECDSA(pk, "")
	if err != nil {
		return err
	}
	as.Set(string(key), account.Address.String())
	fmt.Printf("Migrated %s key to encrypted key store with empty passphrase.\n", key)
	return nil
}

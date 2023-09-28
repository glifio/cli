/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/briandowns/spinner"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	init2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/glifio/cli/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newMsigCmd represents the new-msig command
var newMsigCmd = &cobra.Command{
	Use:   "new-msig",
	Short: "Create an multisig account for the owner",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			logFatalf("Failed to instantiate eth client %s", err)
		}
		defer closer()

		as := util.AgentStore()

		ownerAddr, _, err := as.GetAddrs(util.OwnerKey, nil)
		panicIfKeyExists(util.OwnerKey, ownerAddr, err)

		_, ownerProposerFilAddr, err := as.GetAddrs(util.OwnerProposerKey, lapi)
		if err != nil {
			logFatal(err)
		}

		_, ownerApproverFilAddr, err := as.GetAddrs(util.OwnerApproverKey, lapi)
		if err != nil {
			logFatal(err)
		}

		sendAddr, err := lapi.WalletDefaultAddress(ctx)
		if err != nil {
			log.Fatal(err)
		}

		var required uint64 = 2

		addrs := []address.Address{ownerProposerFilAddr, ownerApproverFilAddr}

		d := abi.ChainEpoch(0) // length of period over which funds unlock

		intVal := ltypes.NewInt(0) // initial value

		gp := ltypes.NewInt(0) // gp = gas price, unused

		proto, err := lapi.MsigCreate(ctx, required, addrs, d, intVal, sendAddr, gp)
		if err != nil {
			logFatal(err)
		}

		msg := &ltypes.Message{
			From:   proto.Message.From,
			To:     proto.Message.To,
			Method: proto.Message.Method,
			Params: proto.Message.Params,
			Value:  proto.Message.Value,
		}

		smsg, err := lapi.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			logFatal(err)
		}
		cid := smsg.Cid()
		fmt.Println("Creating msig, message CID:", cid)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		// wait for it to get mined into a block
		mw, err := lapi.StateWaitMsg(ctx, smsg.Cid(), 0, 900, true)
		if err != nil {
			s.Stop()
			log.Fatal(err)
		}
		s.Stop()

		// check it executed successfully
		if mw.Receipt.ExitCode.IsError() {
			logFatal("msig create failed!")
		}

		var execreturn init2.ExecReturn
		if err := execreturn.UnmarshalCBOR(bytes.NewReader(mw.Receipt.Return)); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Multisig ID:", execreturn.IDAddress)
		fmt.Println("Multisig Robust Address:", execreturn.RobustAddress)

		as.Set(string(util.OwnerKey), execreturn.RobustAddress.String())

		if err := viper.WriteConfig(); err != nil {
			logFatal(err)
		}

		listAddresses(lapi)
		log.Println()
		log.Println("Please make sure to fund your Owner Proposer/Approver Addresses with FIL before creating an Agent")
	},
}

func init() {
	walletCmd.AddCommand(newMsigCmd)
}

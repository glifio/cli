/*
Copyright © 2023 Glif LTD
*/
package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/glifio/go-pools/abigen"
	"github.com/glifio/go-pools/constants"
	"github.com/glifio/go-pools/econ"
	"github.com/glifio/go-pools/rpc"
	"github.com/glifio/go-pools/sdk"
	"github.com/glifio/go-pools/terminate"
	"github.com/glifio/go-pools/util"
	"github.com/glifio/go-pools/vc"
	"github.com/jimpick/go-ethereum/common"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"github.com/ttacon/chalk"
)

var agentInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get the info associated with your Agent",
	Run: func(cmd *cobra.Command, args []string) {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			logFatal(err)
		}
		defer closer()

		agentAddr, err := getAgentAddressWithFlags(cmd)
		if err != nil {
			logFatal(err)
		}

		agentAddrEthType, err := ethtypes.ParseEthAddress(agentAddr.String())
		if err != nil {
			logFatal(err)
		}

		agentAddrDel, err := agentAddrEthType.ToFilecoinAddress()
		if err != nil {
			logFatal(err)
		}

		agentID, _, _, _, err := basicInfo(cmd.Context(), agentAddr, agentAddrDel, lapi, s)
		if err != nil {
			logFatal(err)
		}

		agentData, ats, err := econInfo(cmd.Context(), agentAddr, agentID, lapi, s)
		if err != nil {
			logFatal(err)
		}

		err = agentHealth(cmd.Context(), agentAddr, agentData, ats, s)
		if err != nil {
			logFatal(err)
		}
	},
}

func basicInfo(ctx context.Context, agent common.Address, agentDel address.Address, lapi *api.FullNodeStruct, s *spinner.Spinner) (
	agentID *big.Int,
	agentFILIDAddr address.Address,
	agVersion uint8,
	ntwVersion uint8,
	err error,
) {
	query := PoolsSDK.Query()

	tasks := []util.TaskFunc{
		func() (interface{}, error) {
			return query.AgentID(ctx, agent)
		},
		func() (interface{}, error) {
			return lapi.StateLookupID(ctx, agentDel, types.EmptyTSK)
		},
		func() (interface{}, error) {
			ver, ntwVer, err := query.AgentVersion(ctx, agent)
			return []interface{}{ver, ntwVer}, err
		},
		func() (interface{}, error) {
			return query.AgentOwner(ctx, agent)
		},
		func() (interface{}, error) {
			return query.AgentOperator(ctx, agent)
		},
		func() (interface{}, error) {
			return query.AgentRequester(ctx, agent)
		},
		func() (interface{}, error) {
			agentID, err := query.AgentID(ctx, agent)
			if err != nil {
				return nil, err
			}
			return query.MinerRegistryAgentMinersList(ctx, agentID, nil)
		},
	}
	results, err := util.Multiread(tasks)
	if err != nil {
		return common.Big0, address.Undef, 0, 0, err
	}

	agentID = results[0].(*big.Int)
	agentFILIDAddr = results[1].(address.Address)
	versionResults := results[2].([]interface{})
	agVersion = versionResults[0].(uint8)
	ntwVersion = versionResults[1].(uint8)
	owner := results[3].(common.Address)
	operator := results[4].(common.Address)
	requester := results[5].(common.Address)
	agentMiners := results[6].([]address.Address)

	goodVersion := agVersion == ntwVersion

	s.Stop()

	versionCopy := fmt.Sprintf("%v ✅", agVersion)
	if !goodVersion {
		versionCopy = fmt.Sprintf("Please upgrade Agent ❌. Your version: %v, latest: %v", agVersion, ntwVersion)
	}

	basicInfoKeys := []string{
		"Agent 0x Addr",
		"Agent f4 Addr",
		"Agent f0 Addr",
		"Agent GLIF ID",
		"Agent Owner",
		"Agent Operator",
		"Agent ADO Requester",
		"Agent Miners",
		"Version",
	}

	basicInfoValues := []string{
		agent.String(),
		agentDel.String(),
		agentFILIDAddr.String(),
		agentID.String(),
		owner.String(),
		operator.String(),
		requester.String(),
		fmt.Sprintf("%v", len(agentMiners)),
		versionCopy,
	}

	generateHeader("BASIC INFO")
	printTable(basicInfoKeys, basicInfoValues)

	s.Start()

	return agentID, agentFILIDAddr, agVersion, ntwVersion, nil
}

func econInfo(ctx context.Context, agent common.Address, agentID *big.Int, lapi *api.FullNodeStruct, s *spinner.Spinner) (*vc.AgentData, terminate.PreviewAgentTerminationSummary, error) {
	query := PoolsSDK.Query()

	adoCloser, err := PoolsSDK.Extern().ConnectAdoClient(ctx)
	if err != nil {
		return nil, terminate.PreviewAgentTerminationSummary{}, err
	}
	defer adoCloser()

	agentData, err := rpc.ADOClient.AgentData(context.Background(), agent)
	if err != nil {
		return nil, terminate.PreviewAgentTerminationSummary{}, err
	}

	tasks := []util.TaskFunc{
		func() (interface{}, error) {
			return query.AgentLiquidAssets(ctx, agent, nil)
		},
		func() (interface{}, error) {
			return PoolsSDK.Query().InfPoolAgentMaxBorrow(ctx, agent, agentData)
		},
		func() (interface{}, error) {
			return PoolsSDK.Query().AgentPreviewTerminationQuick(ctx, agent)
		},
		func() (interface{}, error) {
			amountOwed, err := query.AgentInterestOwed(ctx, agent, nil)
			if err != nil {
				return nil, err
			}
			return amountOwed, nil
		},
		func() (interface{}, error) {
			lvl, cap, err := query.InfPoolGetAgentLvl(ctx, agentID)
			if err != nil {
				return nil, err
			}
			return []interface{}{lvl, cap}, nil
		},
	}

	results, err := util.Multiread(tasks)
	if err != nil {
		return nil, terminate.PreviewAgentTerminationSummary{}, err
	}
	assets := results[0].(*big.Int)
	borrowNow := results[1].(*big.Int)
	ats := results[2].(terminate.PreviewAgentTerminationSummary)
	liquidationValue := ats.LiquidationValue()
	recoveryRate := ats.RecoveryRate()
	amountOwed := results[3].(*big.Int)
	lvlAndCap := results[4].([]interface{})
	lvl := lvlAndCap[0].(*big.Int)
	cap := lvlAndCap[1].(float64)

	// here borrowMax does not ignores existing principal, so we add back existing principal to compute the max borrow (that does not account for existing principal)
	borrowMaxDTE := sdk.ComputeMaxDTECap(agentData.AgentValue, agentData.Principal)
	borrowMaxDTE.Add(borrowMaxDTE, agentData.Principal)

	borrowMaxLTV := sdk.ComputeMaxLTVCap(liquidationValue, agentData.Principal, recoveryRate)
	borrowMaxLTV.Add(borrowMaxLTV, agentData.Principal)

	borrowMax := big.NewInt(0)
	// take the minimum between DTE and LTV limits
	if borrowMaxDTE.Cmp(borrowMaxLTV) > 0 {
		borrowMax = borrowMaxLTV
	} else {
		borrowMax = borrowMaxDTE
	}

	nullCred, err := vc.NullishVerifiableCredential(*agentData)
	if err != nil {
		return nil, terminate.PreviewAgentTerminationSummary{}, err
	}

	rate, err := query.InfPoolGetRate(ctx, *nullCred)
	if err != nil {
		return nil, terminate.PreviewAgentTerminationSummary{}, err
	}

	wpr := new(big.Float).Mul(new(big.Float).SetInt(rate), big.NewFloat(constants.EpochsInWeek))

	apr := new(big.Float).Mul(new(big.Float).SetInt(rate), big.NewFloat(constants.EpochsInYear))
	apr.Quo(apr, big.NewFloat(1e34))

	weeklyEarnings := new(big.Int).Mul(agentData.ExpectedDailyRewards, big.NewInt(7))

	weeklyPmt := new(big.Float).Mul(new(big.Float).SetInt(agentData.Principal), wpr)
	weeklyPmt.Quo(weeklyPmt, big.NewFloat(1e54))

	equity := new(big.Int).Sub(agentData.AgentValue, agentData.Principal)
	dte := econ.DebtToEquityRatio(agentData.Principal, equity)

	dailyFees := rate.Mul(rate, big.NewInt(constants.EpochsInDay))
	dailyFees.Mul(dailyFees, agentData.Principal)
	dailyFees.Div(dailyFees, constants.WAD)

	s.Stop()

	generateHeader("ECON INFO")

	printTable([]string{
		"Borrow now",
		"Max borrow",
		// "Agent's max withdraw",
	}, []string{
		fmt.Sprintf("%0.09f FIL", util.ToFIL(borrowNow)),
		fmt.Sprintf("%0.09f FIL", util.ToFIL(borrowMax)),
		// fmt.Sprintf("%0.09f FIL", util.ToFIL(maxWithdraw)),
	})

	printTable([]string{
		"Liquidation value",
		"Recovery rate",
	}, []string{
		fmt.Sprintf("\033[1m%0.09f FIL\033[0m", util.ToFIL(liquidationValue)),
		fmt.Sprintf("%0.03f%%", bigIntAttoToPercent(ats.RecoveryRate())),
	})

	if lvl.Cmp(big.NewInt(0)) == 0 && chainID == constants.MainnetChainID {
		fmt.Println()
		fmt.Println(chalk.Bold.TextStyle("Please open up a request for quota on GitHub: https://tinyurl.com/glif-entry-request"))
	}
	if agentData.Principal.Cmp(big.NewInt(0)) == 0 {
		nothingBorrowedKeys := []string{
			"Total borrowed",
			"Agent's liquid FIL",
			"Agent's total FIL",
			"Agent's equity",
			"Agent's expected weekly earnings",
			"Agent's quota",
		}

		nothingBorrowedValues := []string{
			"0 FIL",
			fmt.Sprintf("%0.08f FIL", util.ToFIL(assets)),
			fmt.Sprintf("%0.08f FIL", util.ToFIL(agentData.AgentValue)),
			fmt.Sprintf("%0.08f FIL", util.ToFIL(equity)),
			fmt.Sprintf("%0.08f FIL", util.ToFIL(weeklyEarnings)),
			fmt.Sprintf("%.03f FIL", cap),
		}
		printTable(nothingBorrowedKeys, nothingBorrowedValues)
	} else {
		somethingBorrowedKeys := []string{
			"Total borrowed",
			"You current owe",
			"Current borrow APR",
			"Your weekly payment",
			"Quota",
		}

		somethingBorrowedValues := []string{
			fmt.Sprintf("%0.09f FIL", util.ToFIL(agentData.Principal)),
			fmt.Sprintf("%0.09f FIL", util.ToFIL(amountOwed)),
			fmt.Sprintf("%.03f%%", apr),
			fmt.Sprintf("%0.09f FIL", weeklyPmt),
			fmt.Sprintf("%.03f FIL", cap),
		}
		printTable(somethingBorrowedKeys, somethingBorrowedValues)

		dti := new(big.Int).Div(dailyFees, agentData.ExpectedDailyRewards)

		ltv := ats.LTV(agentData.Principal)

		coreEconKeys := []string{
			"Liquid FIL",
			"Total FIL",
			"Equity",
			"Expected weekly earnings",
			"Debt-to-liquidation-value (LTV)",
			"Debt-to-equity (DTE)",
			"Debt-to-income (DTI)",
		}

		coreEconValues := []string{
			fmt.Sprintf("%0.08f FIL", util.ToFIL(assets)),
			fmt.Sprintf("%0.08f FIL", util.ToFIL(agentData.AgentValue)),
			fmt.Sprintf("%0.08f FIL", util.ToFIL(equity)),
			fmt.Sprintf("%0.08f FIL", util.ToFIL(weeklyEarnings)),
			fmt.Sprintf("%0.03f%% (must stay below %0.00f%%)", bigIntAttoToPercent(ltv), bigIntAttoToPercent(constants.MAX_LTV)),
			fmt.Sprintf("%0.03f%% (must stay below %0.00f%%)", dte.Mul(dte, big.NewFloat(100)), bigIntAttoToPercent(constants.MAX_DTE)),
			fmt.Sprintf("%0.03f%% (must stay below %0.00f%%)", bigIntAttoToPercent(dti), bigIntAttoToPercent(constants.MAX_DTI)),
		}

		printTable(coreEconKeys, coreEconValues)
	}

	s.Start()

	return agentData, ats, nil
}

func bigIntAttoToPercent(atto *big.Int) *big.Float {
	return new(big.Float).Mul(util.ToFIL(atto), big.NewFloat(100))
}

func printTable(keys []string, values []string) {
	// here we hacky get the same width for all separate tables in the info command by making the first row have a long width
	tbl := table.New("                                    ", "")

	for i, k := range keys {
		tbl.AddRow(k, values[i])
	}

	tbl.Print()
}

func agentHealth(ctx context.Context, agent common.Address, agentData *vc.AgentData, ats terminate.PreviewAgentTerminationSummary, s *spinner.Spinner) error {
	query := PoolsSDK.Query()

	tasks := []util.TaskFunc{
		func() (interface{}, error) {
			return query.AgentAdministrator(ctx, agent)
		},

		func() (interface{}, error) {
			return query.AgentDefaulted(ctx, agent)
		},
		func() (interface{}, error) {
			return query.AgentFaultyEpochStart(ctx, agent)
		},
		func() (interface{}, error) {
			return query.DefaultEpoch(ctx)
		},
		func() (interface{}, error) {
			return query.InfPoolGetAccount(ctx, agent, nil)
		},
	}

	results, err := util.Multiread(tasks)
	if err != nil {
		return err
	}

	agentAdmin := results[0].(common.Address)
	defaulted := results[1].(bool)
	faultySectorStart := results[2].(*big.Int)
	defaultEpoch := results[3].(*big.Int)
	account := results[4].(abigen.Account)

	overLTV := ats.LTV(agentData.Principal).Cmp(constants.MAX_LTV) > 0

	weekOneDeadline := new(big.Int).Add(defaultEpoch, big.NewInt(constants.EpochsInWeek*2))

	weekOneDeadlineTime := util.EpochHeightToTimestamp(weekOneDeadline, query.ChainID())
	defaultEpochTime := util.EpochHeightToTimestamp(defaultEpoch, query.ChainID())
	epochsPaidTime := util.EpochHeightToTimestamp(account.EpochsPaid, query.ChainID())

	s.Stop()

	generateHeader("HEALTH")
	fmt.Println()
	// check to see we're still in good standing wrt making our weekly payment
	owesPmt := account.Principal.Cmp(big.NewInt(0)) > 0
	badPmtStatus := owesPmt && account.EpochsPaid.Cmp(weekOneDeadline) < 1
	badFaultStatus := faultySectorStart.Cmp(big.NewInt(0)) > 0

	faultRatio := big.NewFloat(0)

	// check to see if we have faulty sectors (regardless of the Agent's state)
	pendingBadFaultStatus := false
	if agentData.LiveSectors.Int64() > 0 {
		faultRatio = new(big.Float).Quo(new(big.Float).SetInt(agentData.FaultySectors), new(big.Float).SetInt(agentData.LiveSectors))
		// faulty sectors exist over the limit
		if faultRatio.Cmp(constants.FAULTY_SECTOR_TOLERANCE) > 0 {
			pendingBadFaultStatus = true
		}
	}

	// convert faults into percentage for logging
	faultRatio = faultRatio.Mul(faultRatio, big.NewFloat(100))
	// convert limit into percentage for logging
	limit := new(big.Float).Mul(constants.FAULTY_SECTOR_TOLERANCE, big.NewFloat(100))

	if !badPmtStatus && !badFaultStatus && !pendingBadFaultStatus && !overLTV {
		fmt.Printf("Status healthy 🟢\n")
		if owesPmt {
			fmt.Printf("Your account owes its weekly payment (`to-current`) within the next: %s (by epoch # %s)\n", formatSinceDuration(weekOneDeadlineTime, epochsPaidTime), weekOneDeadline)
		}
	} else {
		fmt.Println(chalk.Bold.TextStyle("Status unhealthy 🔴"))
	}

	if overLTV {
		fmt.Printf("WARNING: Your Agent is over the LTV limit of %0.00f%%\n", bigIntAttoToPercent(constants.MAX_LTV))
		fmt.Printf("Your Agent must pay down its debt or increase its collateral to avoid liquidation\n")
		fmt.Printf("Contact the GLIF team as soon as possible\n")
	}

	if badPmtStatus {
		fmt.Println("You are late on your weekly payment")
		fmt.Printf("Your account *must* make a payment to-current within the next: %s (by epoch # %s)\n", formatSinceDuration(defaultEpochTime, epochsPaidTime), defaultEpoch)
	}

	// since we have to report faulty sectors when the Agent is overLTV, we only display this message if the Agent is not overLTV AND has faulty sectors
	if badFaultStatus && !overLTV {
		chainHeight, err := query.ChainHeight(ctx)
		if err != nil {
			return err
		}

		consecutiveFaultEpochTolerance, err := query.MaxConsecutiveFaultEpochs(ctx)
		if err != nil {
			return err
		}

		consecutiveFaultEpochs := new(big.Int).Sub(chainHeight, faultySectorStart)

		liableForFaultySectorDefault := consecutiveFaultEpochs.Cmp(consecutiveFaultEpochTolerance) >= 0

		if liableForFaultySectorDefault {
			fmt.Printf("You are at risk of liquidation due to consecutive faulty sectors - recover your sectors as soon as possible\n")
			fmt.Printf("Faulty sector start epoch: %v\n", faultySectorStart)
		} else {
			epochsBeforeZeroTolerance := new(big.Int).Sub(consecutiveFaultEpochTolerance, consecutiveFaultEpochs)
			fmt.Printf("WARNING: You are approaching risk of liquidation due to consecutive faulty sectors\n")
			fmt.Printf("With %v more consecutive epochs of faulty sectors, you will be at risk of liquidation\n", epochsBeforeZeroTolerance)
		}
	} else if pendingBadFaultStatus {
		fmt.Printf("WARNING: Your Agent has one or more miners with faulty sectors - recover your sectors as soon as possible\n")
		fmt.Printf("Faulty sector ratio: %.02f%%\n", faultRatio)
		fmt.Printf("Faulty sector ratio limit: %v%%\n", limit.String())
	}

	printTable([]string{
		"Agent's administrator",
		"Agent in default",
	}, []string{
		agentAdmin.String(),
		fmt.Sprintf("%t", defaulted),
	})
	fmt.Println()

	return nil
}

func formatSinceDuration(t1 time.Time, t2 time.Time) string {
	d := t2.Sub(t1).Round(time.Minute)

	var parts []string

	weeks := int(d.Hours()) / (24 * 7)
	d -= time.Duration(weeks) * 7 * 24 * time.Hour
	if weeks > 1 {
		parts = append(parts, fmt.Sprintf("%d weeks", weeks))
	} else if weeks == 1 {
		parts = append(parts, fmt.Sprintf("%d week", weeks))
	}

	days := int(d.Hours()) / 24
	d -= time.Duration(days) * 24 * time.Hour
	if days > 1 {
		parts = append(parts, fmt.Sprintf("%d days", days))
	} else if days == 1 {
		parts = append(parts, fmt.Sprintf("%d day", days))
	}

	h := d / time.Hour
	d -= h * time.Hour
	parts = append(parts, fmt.Sprintf("%02d hours", h))

	m := d / time.Minute
	parts = append(parts, fmt.Sprintf("and %02d minutes", m))

	return strings.Join(parts, " ")
}

func generateHeader(title string) {
	fmt.Println()
	fmt.Printf("\033[1m%s\033[0m\n", chalk.Underline.TextStyle(title))
}

func init() {
	agentCmd.AddCommand(agentInfoCmd)
	agentInfoCmd.Flags().String("agent-addr", "", "Agent address")
}

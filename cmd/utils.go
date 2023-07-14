package cmd

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"log"
	"math/big"
	"runtime"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-address"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/filecoin-project/lotus/api"
	lotusapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/types"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/glifio/cli/util"
	denoms "github.com/glifio/go-pools/util"
	"github.com/spf13/cobra"
)

var ExitCode int

func Exit(code int) {
	ExitCode = code
	runtime.Goexit()
}

func logExit(code int, msg string) {
	log.Println(msg)
	Exit(code)
}

func logFatal(arg interface{}) {
	log.Println(arg)
	Exit(1)
}

func logFatalf(format string, args ...interface{}) {
	log.Printf(format, args...)
	Exit(1)
}

func ParseAddressToNative(ctx context.Context, addr string) (address.Address, error) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return address.Undef, err
	}
	defer closer()

	// user passed 0x addr, convert to f4
	if strings.HasPrefix(addr, "0x") {
		ethAddr, err := ethtypes.ParseEthAddress(addr)
		if err != nil {
			return address.Undef, err
		}

		return ethAddr.ToFilecoinAddress()
	}

	// user passed f0, f1, f2, f3, or f4
	filAddr, err := address.NewFromString(addr)
	if err != nil {
		return address.Undef, err
	}

	// Note that in testing, sending to an ID actor address works ok but we still block it, as this isn't intended good behavior (passing ID addrs as representations of 0x style EVM addrs)
	if err := checkIDNotEVMActorType(ctx, filAddr, lapi); err != nil {
		return address.Undef, err
	}

	return filAddr, nil
}

func ParseAddressToEVM(ctx context.Context, addr string) (common.Address, error) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return common.Address{}, err
	}
	defer closer()

	return parseAddress(ctx, addr, lapi)
}

func ToMinerID(ctx context.Context, addr string) (address.Address, error) {
	minerAddr, err := address.NewFromString(addr)
	if err != nil {
		return address.Undef, err
	}

	if minerAddr.Protocol() == address.ID {
		return minerAddr, nil
	}

	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return address.Undef, err
	}
	defer closer()

	idAddr, err := lapi.StateLookupID(context.Background(), minerAddr, ltypes.EmptyTSK)
	if err != nil {
		return address.Undef, err
	}

	return idAddr, nil
}

// using f0 ID addresses to interact with EVM or EthAccount actor types is forbidden
func checkIDNotEVMActorType(ctx context.Context, filAddr address.Address, lapi api.FullNode) error {
	if filAddr.Protocol() == address.ID {
		actor, err := lapi.StateGetActor(ctx, filAddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		actorCodeEvm, success := actors.GetActorCodeID(actorstypes.Version(actors.LatestVersion), manifest.EvmKey)
		if !success {
			return errors.New("actor code not found")
		}
		if actor.Code.Equals(actorCodeEvm) {
			return errors.New("Cant pass an ID address of an EVM actor")
		}

		actorCodeEthAccount, success := actors.GetActorCodeID(actorstypes.Version(actors.LatestVersion), manifest.EthAccountKey)
		if !success {
			return errors.New("actor code not found")
		}
		if actor.Code.Equals(actorCodeEthAccount) {
			return errors.New("Cant pass an ID address of an Eth Account")
		}
	}

	return nil
}

func parseAddress(ctx context.Context, addr string, lapi lotusapi.FullNode) (common.Address, error) {
	if strings.HasPrefix(addr, "0x") {
		return common.HexToAddress(addr), nil
	}
	// user passed f1, f2, f3, or f4
	filAddr, err := address.NewFromString(addr)

	if err != nil {
		return common.Address{}, err
	}

	if err := checkIDNotEVMActorType(ctx, filAddr, lapi); err != nil {
		return common.Address{}, err
	}

	if filAddr.Protocol() != address.ID && filAddr.Protocol() != address.Delegated {
		filAddr, err = lapi.StateLookupID(ctx, filAddr, types.EmptyTSK)
		if err != nil {
			return common.Address{}, err
		}
	}

	ethAddr, err := ethtypes.EthAddressFromFilecoinAddress(filAddr)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(ethAddr.String()), nil
}

func commonSetupOwnerCall() (common.Address, *ecdsa.PrivateKey, *ecdsa.PrivateKey, error) {
	as := util.AgentStore()
	ks := util.KeyStore()
	// Check if an agent already exists
	agentAddrStr, err := as.Get("address")
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	if agentAddrStr == "" {
		return common.Address{}, nil, nil, errors.New("No agent found. Did you forget to create one?")
	}

	agentAddr := common.HexToAddress(agentAddrStr)

	pk, err := ks.GetPrivate(util.OwnerKey)
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	if pk == nil {
		return common.Address{}, nil, nil, errors.New("Owner key not found. Please check your `keys.toml` file.")
	}

	requesterKey, err := ks.GetPrivate(util.RequestKey)
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	if pk == nil {
		return common.Address{}, nil, nil, errors.New("Requester key not found. Please check your `keys.toml` file.")
	}

	return agentAddr, pk, requesterKey, nil
}

func commonOwnerOrOperatorSetup(cmd *cobra.Command) (common.Address, *ecdsa.PrivateKey, *ecdsa.PrivateKey, error) {
	as := util.AgentStore()
	ks := util.KeyStore()

	opEvm, opFevm, err := ks.GetAddrs(util.OperatorKey)
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	owEvm, owFevm, err := ks.GetAddrs(util.OwnerKey)
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	var pk *ecdsa.PrivateKey
	// if no flag was passed, we just use the operator address by default
	from := cmd.Flag("from").Value.String()
	switch from {
	case "", opEvm.String(), opFevm.String():
		funded, err := as.IsFunded(cmd.Context(), PoolsSDK, opFevm, util.OperatorKeyFunded, opEvm.String())
		if err != nil {
			return common.Address{}, nil, nil, err
		}
		if funded {
			pk, err = ks.GetPrivate(util.OperatorKey)
		} else {
			log.Println("operator not funded, falling back to owner address")
			pk, err = ks.GetPrivate(util.OwnerKey)
		}
		if err != nil {
			return common.Address{}, nil, nil, err
		}
	case owEvm.String(), owFevm.String():
		pk, err = ks.GetPrivate(util.OwnerKey)
	default:
		return common.Address{}, nil, nil, errors.New("invalid from address")
	}
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	agentAddrStr, err := as.Get("address")
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	if agentAddrStr == "" {
		return common.Address{}, nil, nil, errors.New("No agent found. Did you forget to create one?")
	}

	agentAddr := common.HexToAddress(agentAddrStr)

	requesterKey, err := ks.GetPrivate(util.RequestKey)
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	if pk == nil {
		return common.Address{}, nil, nil, errors.New("Requester key not found. Please check your `keys.toml` file.")
	}

	return agentAddr, pk, requesterKey, nil
}

type PoolType uint64

const (
	InfinityPool PoolType = iota
)

var poolNames = map[string]PoolType{
	"infinity-pool": InfinityPool,
}

func parsePoolType(pool string) (*big.Int, error) {
	if pool == "" {
		return common.Big0, errors.New("Invalid pool name")
	}

	poolType, ok := poolNames[pool]
	if !ok {
		return nil, errors.New("invalid pool")
	}

	return big.NewInt(int64(poolType)), nil
}

// parseFILAmount takes a string amount of FIL and returns
// that amount as a *big.Int in attoFIL
func parseFILAmount(amount string) (*big.Int, error) {
	amt, ok := new(big.Float).SetString(amount)
	if !ok {
		return nil, errors.New("invalid amount")
	}

	return denoms.ToAtto(amt), nil
}

func getAgentAddress(cmd *cobra.Command) (common.Address, error) {
	as := util.AgentStore()
	var agentAddrStr string

	if cmd.Flag("agent-addr") != nil && cmd.Flag("agent-addr").Changed {
		agentAddrStr = cmd.Flag("agent-addr").Value.String()
	} else {
		// Check if an agent already exists
		cachedAddr, err := as.Get("address")
		if err != nil {
			return common.Address{}, err
		}

		agentAddrStr = cachedAddr

		if agentAddrStr == "" {
			return common.Address{}, errors.New("Did you forget to create your agent or specify an address? Try `glif agent id --address <address>`")
		}
	}

	return common.HexToAddress(agentAddrStr), nil
}

func getAgentID(cmd *cobra.Command) (*big.Int, error) {
	var agentIDStr string

	if cmd.Flag("agent-id") != nil && cmd.Flag("agent-id").Changed {
		agentIDStr = cmd.Flag("agent-id").Value.String()
	} else {
		as := util.AgentStore()
		storedAgent, err := as.Get("id")
		if err != nil {
			logFatal(err)
		}

		agentIDStr = storedAgent
	}

	agentID := new(big.Int)
	if _, ok := agentID.SetString(agentIDStr, 10); !ok {
		logFatalf("could not convert agent id %s to big.Int", agentIDStr)
	}

	return agentID, nil
}

func AddressesToStrings(addrs []address.Address) []string {
	strs := make([]string, len(addrs))
	for i, addr := range addrs {
		strs[i] = addr.String()
	}
	return strs
}

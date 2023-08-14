package cmd

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	ethaccounts "github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
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
	"github.com/glifio/go-wallet-utils/accounts"
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

func commonSetupOwnerCall() (agentAddr common.Address, ownerWallet accounts.Wallet, ownerAccount accounts.Account, ownerPassphrase string, requesterKey *ecdsa.PrivateKey, err error) {
	as := util.AgentStore()

	_, ownerFilAddr, err := as.GetAddrs(util.OwnerKey, nil)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	agentAddr, wallet, account, passphrase, requesterKey, err := commonOwnerOrOperatorSetup(context.Background(), ownerFilAddr.String())
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	return agentAddr, wallet, account, passphrase, requesterKey, nil
}

func commonOwnerOrOperatorSetup(ctx context.Context, from string) (agentAddr common.Address, wallet accounts.Wallet, account accounts.Account, passphrase string, requesterKey *ecdsa.PrivateKey, err error) {
	err = checkWalletMigrated()
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	as := util.AgentStore()
	ks := util.KeyStore()
	backends := []ethaccounts.Backend{}
	backends = append(backends, ks)
	manager := accounts.NewManager(&ethaccounts.Config{InsecureUnlockAllowed: false}, backends, []accounts.Backend{})

	opEvm, opFevm, err := as.GetAddrs(util.OperatorKey, nil)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	// FIXME: Handle Fil addresses
	owEvm, owFevm, err := as.GetAddrs(util.OwnerKey, nil)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	var fromEthAddress common.Address
	var fromFilAddress address.Address

	// if no flag was passed, we just use the operator address by default
	// from := cmd.Flag("from").Value.String()
	switch from {
	case "", opEvm.String(), opFevm.String():
		funded, err := as.IsFunded(ctx, PoolsSDK, opFevm, util.OperatorKeyFunded, opEvm.String())
		if err != nil {
			return common.Address{}, nil, accounts.Account{}, "", nil, err
		}
		if funded {
			fromEthAddress = opEvm
		} else {
			log.Println("operator not funded, falling back to owner address")
			fromEthAddress = owEvm
		}
		if err != nil {
			return common.Address{}, nil, accounts.Account{}, "", nil, err
		}
	case owEvm.String(), owFevm.String():
		if owFevm.Protocol() == address.Delegated {
			fromEthAddress = owEvm
		} else {
			fromFilAddress = owFevm
		}
	default:
		return common.Address{}, nil, accounts.Account{}, "", nil, errors.New("invalid from address")
	}
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	agentAddr, err = getAgentAddress()
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	if !fromFilAddress.Empty() {
		account = accounts.Account{FilAddress: fromFilAddress}
	} else {
		account = accounts.Account{EthAccount: ethaccounts.Account{Address: fromEthAddress}}
	}
	wallet, err = manager.Find(account)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	var envSet bool
	var message string
	if fromFilAddress.Empty() {
		if fromEthAddress == owEvm {
			passphrase, envSet = os.LookupEnv("GLIF_OWNER_PASSPHRASE")
			message = "Owner key passphrase"
		} else if fromEthAddress == opEvm {
			passphrase, envSet = os.LookupEnv("GLIF_OPERATOR_PASSPHRASE")
			message = "Operator key passphrase"
		}
		if !envSet {
			err = ks.Unlock(account.EthAccount, "")
			if err != nil {
				prompt := &survey.Password{Message: message}
				survey.AskOne(prompt, &passphrase)
				if passphrase == "" {
					return common.Address{}, nil, accounts.Account{}, "", nil, fmt.Errorf("Aborted")
				}
			}
		}
	}

	requesterKey, err = getRequesterKey(as, ks)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, "", nil, err
	}

	return agentAddr, wallet, account, passphrase, requesterKey, nil
}

func getRequesterKey(as *util.AgentStorage, ks *keystore.KeyStore) (*ecdsa.PrivateKey, error) {
	requesterAddr, _, err := as.GetAddrs(util.RequestKey, nil)
	if err != nil {
		return nil, err
	}
	requesterAccount := ethaccounts.Account{Address: requesterAddr}
	requesterKeyJSON, err := ks.Export(requesterAccount, "", "")
	if err != nil {
		return nil, err
	}
	rk, err := keystore.DecryptKey(requesterKeyJSON, "")
	if err != nil {
		return nil, err
	}
	return rk.PrivateKey, nil
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

func getAgentAddress() (common.Address, error) {
	as := util.AgentStore()

	// Check if an agent already exists
	agentAddrStr, err := as.Get("address")
	if err != nil {
		return common.Address{}, err
	}

	if agentAddrStr == "" {
		return common.Address{}, errors.New("Did you forget to create your agent or specify an address? Try `glif agent id --address <address>`")
	}

	return common.HexToAddress(agentAddrStr), nil
}

func getAgentAddressWithFlags(cmd *cobra.Command) (common.Address, error) {
	if cmd.Flag("agent-addr") != nil && cmd.Flag("agent-addr").Changed {
		agentAddrStr := cmd.Flag("agent-addr").Value.String()
		return common.HexToAddress(agentAddrStr), nil
	} else {
		return getAgentAddress()
	}
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

func checkWalletMigrated() error {
	as := util.AgentStore()
	ksLegacy := util.KeyStoreLegacy()

	notMigratedError := fmt.Errorf("wallet not migrated to encrypted keystore. Please run \"glif wallet migrate\"")

	keys := []util.KeyType{
		util.OwnerKey,
		util.OperatorKey,
		util.RequestKey,
	}

	for _, key := range keys {
		newAddr, newFilAddr, err := as.GetAddrs(key, nil)
		if err != nil {
			return err
		}
		if util.IsZeroAddress(newAddr) && newFilAddr.Empty() {
			oldAddr, _, err := ksLegacy.GetAddrs(key)
			if err != nil {
				return err
			}
			if util.IsZeroAddress(oldAddr) {
				return fmt.Errorf("missing %s key in legacy keys.toml", string(key))
			}
			return notMigratedError
		}
	}

	return nil
}

func checkUnencryptedPrivateKeys() error {
	ksLegacy := util.KeyStoreLegacy()

	keys := []util.KeyType{
		util.OwnerKey,
		util.OperatorKey,
		util.RequestKey,
	}

	for _, key := range keys {
		pk, err := ksLegacy.Get(string(key))
		if err != nil {
			return fmt.Errorf("error checking private key %s: %w", string(key), err)
		}
		if pk != "" {
			return fmt.Errorf("unencrypted keys found in legacy keys.toml after migration. Remove to improve security.")
		}
	}

	return nil
}

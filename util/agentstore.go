package util

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/glifio/go-pools/types"
)

type KeyType string

const (
	OwnerKey          KeyType = "owner"
	OperatorKey       KeyType = "operator"
	RequestKey        KeyType = "request"
	OperatorKeyFunded KeyType = "opkeyf"
	OwnerKeyFunded    KeyType = "ownkeyf"
)

type AgentStorage struct {
	*Storage
}

var agentStore *AgentStorage

func AgentStore() *AgentStorage {
	return agentStore
}

func NewAgentStore(filename string) error {
	agentDefault := map[string]string{
		"id":                "",
		"address":           "",
		"tx":                "",
		string(OwnerKey):    "",
		string(OperatorKey): "",
	}

	s, err := NewStorage(filename, agentDefault)
	if err != nil {
		return err
	}

	agentStore = &AgentStorage{s}

	return nil
}

func (a *AgentStorage) IsFunded(ctx context.Context, psdk types.PoolsSDK, caller address.Address, keytype KeyType, key string) (bool, error) {
	switch keytype {
	case OperatorKeyFunded, OwnerKeyFunded:
		f, ok := a.data[mapkey(keytype, key)]
		if !ok {
			lapi, closer, err := psdk.Extern().ConnectLotusClient()
			if err != nil {
				return false, err
			}
			defer closer()

			bal, err := lapi.WalletBalance(ctx, caller)
			if err != nil {
				return false, err
			}
			if bal.Cmp(big.NewInt(0)) > 0 {
				a.SetFunded(keytype, key, true)
				return true, nil
			}
			return false, nil
		}

		return strconv.ParseBool(f)
	default:
		return false, fmt.Errorf("not supported key type for funded operation")
	}
}

func (a *AgentStorage) SetFunded(keytype KeyType, key string, funded bool) error {
	switch keytype {
	case OperatorKeyFunded, OwnerKeyFunded:
		return a.Set(mapkey(keytype, key), strconv.FormatBool(funded))
	default:
		return fmt.Errorf("not supported key type for funded operation")
	}
}

func mapkey(keytype KeyType, key string) string {
	return fmt.Sprintf("%s-%s", string(keytype), key)
}

func (a *AgentStorage) GetAddrs(key KeyType, lapi *api.FullNodeStruct) (common.Address, address.Address, error) {
	value := a.data[string(key)]

	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	if re.MatchString(value) {
		evmAddress := common.HexToAddress(value)

		delegated, err := DelegatedFromEthAddr(evmAddress)
		if err != nil {
			return evmAddress, address.Address{}, err
		}

		return evmAddress, delegated, nil
	} else {
		filAddr, err := address.NewFromString(value)
		if err != nil {
			return common.Address{}, address.Address{}, err
		}
		if lapi == nil {
			return common.Address{}, filAddr, nil
		}
		idAddr, err := lapi.StateLookupID(context.Background(), filAddr, ltypes.EmptyTSK)
		if err != nil {
			if err.Error() == "actor not found" {
				// Actor hasn't been funded yet
				return common.Address{}, filAddr, nil
			}
			return common.Address{}, address.Address{}, err
		}
		ethAddrLotus, err := ethtypes.EthAddressFromFilecoinAddress(idAddr)
		if err != nil {
			return common.Address{}, address.Address{}, err
		}
		ethAddr := common.HexToAddress(ethAddrLotus.String())

		return ethAddr, filAddr, nil
	}
}

func DelegatedFromEthAddr(addr common.Address) (address.Address, error) {
	fevmAddr, err := ethtypes.ParseEthAddress(addr.String())
	if err != nil {
		return address.Address{}, err
	}

	return fevmAddr.ToFilecoinAddress()
}

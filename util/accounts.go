package util

import (
	"github.com/filecoin-project/go-address"
	"github.com/jimpick/go-ethereum/common"
)

type AccountsStorage struct {
	*Storage
}

var accountsStore *AccountsStorage

func AccountsStore() *AccountsStorage {
	return accountsStore
}

func NewAccountsStore(filename string) error {
	accountsDefault := map[string]string{}

	s, err := NewStorage(filename, accountsDefault, true)
	if err != nil {
		return err
	}

	accountsStore = &AccountsStorage{s}

	return nil
}

func (a *AccountsStorage) GetAddrs(key string) (common.Address, address.Address, error) {
	addr, ok := a.data[key]
	if !ok || addr == "" {
		return common.Address{}, address.Address{}, &ErrKeyNotFound{key}
	}
	evmAddress := common.HexToAddress(addr)

	delegated, err := DelegatedFromEthAddr(evmAddress)
	if err != nil {
		return evmAddress, address.Address{}, err
	}

	return evmAddress, delegated, nil
}

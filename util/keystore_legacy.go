package util

import (
	"crypto/ecdsa"

	"github.com/filecoin-project/go-address"
	"github.com/jimpick/go-ethereum/common"
	"github.com/jimpick/go-ethereum/common/hexutil"
	"github.com/jimpick/go-ethereum/crypto"
)

type KeyStorageLegacy struct {
	*Storage
}

var keyStoreLegacy *KeyStorageLegacy

func KeyStoreLegacy() *KeyStorageLegacy {
	return keyStoreLegacy
}

func NewKeyStoreLegacy(filename string) error {
	keyDefault := map[string]string{
		string(OwnerKey):    "",
		string(OperatorKey): "",
		string(RequestKey):  "",
	}

	s, err := NewStorage(filename, keyDefault, false)
	if err != nil {
		return err
	}

	keyStoreLegacy = &KeyStorageLegacy{s}

	return nil
}

func (s *KeyStorageLegacy) GetPrivate(key KeyType) (*ecdsa.PrivateKey, error) {
	pk, ok := s.data[string(key)]
	if !ok {
		return nil, &ErrKeyNotFound{string(key)}
	}

	pkECDSA, err := crypto.HexToECDSA(pk)
	if err != nil {
		return nil, err
	}

	return pkECDSA, nil
}

func (s *KeyStorageLegacy) GetAddrs(key KeyType) (common.Address, address.Address, error) {
	pk, ok := s.data[string(key)]
	if !ok {
		return common.Address{}, address.Address{}, &ErrKeyNotFound{string(key)}
	}

	if pk == "" {
		return common.Address{}, address.Address{}, &ErrKeyNotFound{string(key)}
	}

	pkECDSA, err := crypto.HexToECDSA(pk)
	if err != nil {
		return common.Address{}, address.Address{}, err
	}

	return DeriveAddrFromPk(pkECDSA)
}

func (s *KeyStorageLegacy) SetKey(key KeyType, pk *ecdsa.PrivateKey) error {
	pkStr := hexutil.Encode(crypto.FromECDSA(pk))[2:]
	err := s.Set(string(key), pkStr)

	return err
}

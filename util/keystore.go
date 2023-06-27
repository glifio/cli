package util

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
)

type KeyType string

const (
	OwnerKey    KeyType = "owner"
	OperatorKey KeyType = "operator"
	RequestKey  KeyType = "request"
)

type KeyStorage struct {
	*Storage
}

var keyStore *KeyStorage

func KeyStore() *KeyStorage {
	return keyStore
}

func NewKeyStore(filename string) error {
	keyDefault := map[string]string{
		string(OwnerKey):    "",
		string(OperatorKey): "",
		string(RequestKey):  "",
	}

	s, err := NewStorage(filename, keyDefault)
	if err != nil {
		return err
	}

	keyStore = &KeyStorage{s}

	return nil
}

func (s *KeyStorage) GetPrivate(key KeyType) (*ecdsa.PrivateKey, error) {
	pk, ok := s.data[string(key)]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	pkECDSA, err := crypto.HexToECDSA(pk)
	if err != nil {
		return nil, err
	}

	return pkECDSA, nil
}

// returns
func (s *KeyStorage) GetAddrs(key KeyType) (common.Address, address.Address, error) {
	pk, ok := s.data[string(key)]
	if !ok {
		return common.Address{}, address.Address{}, nil
	}

	if pk == "" {
		return common.Address{}, address.Address{}, nil
	}

	pkECDSA, err := crypto.HexToECDSA(pk)
	if err != nil {
		return common.Address{}, address.Address{}, err
	}

	return DeriveAddrFromPk(pkECDSA)
}

func (s *KeyStorage) SetKey(key KeyType, pk *ecdsa.PrivateKey) error {
	pkStr := hexutil.Encode(crypto.FromECDSA(pk))[2:]
	err := s.Set(string(key), pkStr)

	return err
}

func (s *KeyStorage) SetHardwareWalletKey(key KeyType, deviceName string, appName string, account string, address string) error {
	err := s.Set(string(key)+"-hw-wallet", deviceName+"-"+appName+"-"+account+"-"+address)

	return err
}

func DeriveAddrFromPkString(pk string) (common.Address, address.Address, error) {
	pkECDSA, err := crypto.HexToECDSA(pk)
	if err != nil {
		log.Fatal(err)
	}

	return DeriveAddrFromPk(pkECDSA)
}

func DeriveAddressFromPk(pk *ecdsa.PrivateKey) (common.Address, error) {
	publicKey := pk.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return common.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}

	return crypto.PubkeyToAddress(*publicKeyECDSA), nil
}

func DeriveAddrFromPk(pk *ecdsa.PrivateKey) (common.Address, address.Address, error) {
	publicKey := pk.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return common.Address{}, address.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}

	evmAddr := crypto.PubkeyToAddress(*publicKeyECDSA)

	fevmAddr, err := ethtypes.ParseEthAddress(evmAddr.String())
	if err != nil {
		return common.Address{}, address.Address{}, err
	}

	delegatedAddr, err := fevmAddr.ToFilecoinAddress()
	if err != nil {
		return common.Address{}, address.Address{}, err
	}

	return evmAddr, delegatedAddr, nil
}

func DelegatedFromEthAddr(addr common.Address) (address.Address, error) {
	fevmAddr, err := ethtypes.ParseEthAddress(addr.String())
	if err != nil {
		return address.Address{}, err
	}

	return fevmAddr.ToFilecoinAddress()
}

// IsZeroAddress validate if it's a 0 address
func IsZeroAddress(address common.Address) bool {
	if isEmptyStruct(address) {
		return true
	}
	zeroAddressBytes := common.FromHex("0x0000000000000000000000000000000000000000")
	addressBytes := address.Bytes()
	return reflect.DeepEqual(addressBytes, zeroAddressBytes)
}

// isEmptyStruct checks if a variable is an empty instance of a struct
func isEmptyStruct(s interface{}) bool {
	v := reflect.ValueOf(s)

	// Ensure the variable is a struct
	if v.Kind() != reflect.Struct {
		return false
	}

	// Check if all fields have their zero values
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface()) {
			return false
		}
	}

	return true
}

func TruncateAddr(addr string) string {
	if len(addr) <= 10 {
		return addr
	}

	firstSix := addr[:6]
	lastFour := addr[len(addr)-4:]
	return fmt.Sprintf("%s...%s", firstSix, lastFour)
}

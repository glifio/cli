package fevm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func DeriveAddressFromPk(pk *ecdsa.PrivateKey) (common.Address, error) {
	publicKey := pk.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return common.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}

	return crypto.PubkeyToAddress(*publicKeyECDSA), nil
}

func WriteTx(
	ctx context.Context,
	pk *ecdsa.PrivateKey,
	client *ethclient.Client,
	args []interface{},
	writeTx interface{},
	label string,
) (*types.Transaction, error) {
	chainID, err := client.ChainID(ctx)

	auth, err := bind.NewKeyedTransactorWithChainID(pk, chainID)
	if err != nil {
		log.Fatal(err)
	}

	fromAddress, err := DeriveAddressFromPk(pk)
	if err != nil {
		return nil, err
	}

	nonce, err := Nonce().BumpNonce(fromAddress, 0)
	if err != nil {
		return nil, err
	}

	auth.Nonce = nonce

	// Use reflection to call the writeTx function with the required arguments
	writeTxValue := reflect.ValueOf(writeTx)
	writeTxArgs := []reflect.Value{reflect.ValueOf(auth)}

	argStrings := make([]string, len(args))
	for i, arg := range args {
		writeTxArgs = append(writeTxArgs, reflect.ValueOf(arg))
		argStrings[i] = StringifyArg(arg)
	}

	fmt.Printf("Calling %s, with %s\n", label, strings.Join(argStrings, ", "))

	result := writeTxValue.Call(writeTxArgs)

	// Get the transaction and error from the result
	tx := result[0].Interface().(*types.Transaction)

	fmt.Printf("Transaction: %s", tx.Hash())
	err = nil
	if !result[1].IsNil() {
		err = result[1].Interface().(error)
	}

	return tx, err
}

func StringifyArg(arg interface{}) string {
	val := reflect.ValueOf(arg)
	typ := val.Type()

	// If the argument is a slice, iterate over its elements and stringify each one
	if typ.Kind() == reflect.Slice {
		elemCount := val.Len()
		elemStrings := make([]string, elemCount)
		for i := 0; i < elemCount; i++ {
			elem := val.Index(i).Interface()
			elemStrings[i] = StringifyArg(elem)
		}
		return fmt.Sprintf("[%s]", strings.Join(elemStrings, ", "))
	}

	// Handle other types
	switch arg.(type) {
	case *bind.TransactOpts:
		return fmt.Sprintf("TransactOpts{From: %s, Nonce: %s}", val.FieldByName("From").Interface(), val.FieldByName("Nonce").Interface())
	case common.Address:
		return fmt.Sprintf("Address: %s", arg.(common.Address).Hex())
	default:
		return fmt.Sprintf("%v", arg)
	}
}

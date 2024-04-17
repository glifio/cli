package cmd

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
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
	walletutils "github.com/glifio/go-wallet-utils"
	"github.com/jimpick/go-ethereum/accounts"
	"github.com/jimpick/go-ethereum/accounts/abi/bind"
	"github.com/jimpick/go-ethereum/accounts/keystore"
	"github.com/jimpick/go-ethereum/common"
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

func AddressOrAccountNameToNative(ctx context.Context, addr string) (address.Address, error) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return address.Undef, err
	}
	defer closer()

	re := regexp.MustCompile(`^[tf][0-9]`)
	if re.MatchString(addr) {
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

	// user passed 0x addr or account name, convert to f4
	var ethAddr ethtypes.EthAddress
	if strings.HasPrefix(addr, "0x") {
		ethAddr, err = ethtypes.ParseEthAddress(addr)
		if err != nil {
			return address.Undef, err
		}
		return ethAddr.ToFilecoinAddress()
	} else {
		as := util.AccountsStore()
		_, fevmAddr, err := as.GetAddrs(addr)
		if err != nil {
			return address.Undef, err
		}
		return fevmAddr, nil
	}
}

func AddressOrAccountNameToEVM(ctx context.Context, addr string) (common.Address, error) {
	if strings.HasPrefix(addr, "0x") {
		return common.HexToAddress(addr), nil
	}

	re := regexp.MustCompile(`^[tf][0-9]`)
	if re.MatchString(addr) {
		lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
		if err != nil {
			return common.Address{}, err
		}
		defer closer()

		return parseAddress(ctx, addr, lapi)
	}

	as := util.AccountsStore()
	evmAddr, _, err := as.GetAddrs(addr)
	if err != nil {
		return common.Address{}, err
	}
	return evmAddr, nil
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

func commonSetupOwnerCall() (agentAddr common.Address, auth *bind.TransactOpts, ownerAccount accounts.Account, requesterKey *ecdsa.PrivateKey, err error) {
	return commonOwnerOrOperatorSetup(context.Background(), string(util.OwnerKey))
}

func commonOwnerOrOperatorSetup(ctx context.Context, from string) (agentAddr common.Address, auth *bind.TransactOpts, account accounts.Account, requesterKey *ecdsa.PrivateKey, err error) {
	err = checkWalletMigrated()
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	as := util.AccountsStore()
	ks := util.KeyStore()
	backends := []accounts.Backend{}
	backends = append(backends, ks)
	manager := accounts.NewManager(&accounts.Config{InsecureUnlockAllowed: false}, backends...)

	opEvm, opFevm, err := as.GetAddrs(string(util.OperatorKey))
	if err != nil {
		var e *util.ErrKeyNotFound
		if errors.As(err, &e) {
			return common.Address{}, nil, accounts.Account{}, nil, fmt.Errorf("agent accounts not found in wallet. Setup with: glif wallet create-agent-accounts")
		}
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	owEvm, owFevm, err := as.GetAddrs(string(util.OwnerKey))
	if err != nil {
		var e *util.ErrKeyNotFound
		if errors.As(err, &e) {
			return common.Address{}, nil, accounts.Account{}, nil, fmt.Errorf("agent accounts not found in wallet. Setup with: glif wallet create-agent-accounts")
		}
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	var fromAddress common.Address
	// if no flag was passed, we just use the operator address by default
	switch strings.ToLower(from) {
	case "", strings.ToLower(opEvm.String()), strings.ToLower(opFevm.String()), string(util.OperatorKey):
		funded, err := isFunded(ctx, opFevm)
		if err != nil {
			return common.Address{}, nil, accounts.Account{}, nil, err
		}
		if funded {
			fromAddress = opEvm
		} else {
			log.Println("operator not funded, falling back to owner address")
			fromAddress = owEvm
		}
		if err != nil {
			return common.Address{}, nil, accounts.Account{}, nil, err
		}
	case strings.ToLower(owEvm.String()), strings.ToLower(owFevm.String()), string(util.OwnerKey):
		fromAddress = owEvm
	default:
		return common.Address{}, nil, accounts.Account{}, nil, errors.New("invalid from address")
	}
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	agentAddr, err = getAgentAddress()
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	account = accounts.Account{Address: fromAddress}
	wallet, err := manager.Find(account)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	var passphrase string
	var envSet bool
	var message string
	if fromAddress == owEvm {
		passphrase, envSet = os.LookupEnv("GLIF_OWNER_PASSPHRASE")
		message = "Owner key passphrase"
	} else if fromAddress == opEvm {
		passphrase, envSet = os.LookupEnv("GLIF_OPERATOR_PASSPHRASE")
		message = "Operator key passphrase"
	}
	if !envSet {
		err = ks.Unlock(account, "")
		if err != nil {
			prompt := &survey.Password{Message: message}
			survey.AskOne(prompt, &passphrase)
			if passphrase == "" {
				return common.Address{}, nil, accounts.Account{}, nil, fmt.Errorf("Aborted")
			}
		}
	}

	requesterKey, err = getRequesterKey(as, ks)
	if err != nil {
		return common.Address{}, nil, accounts.Account{}, nil, err
	}

	auth, err = walletutils.NewEthWalletTransactor(wallet, &account, passphrase, big.NewInt(chainID))
	if err != nil {
		logFatal(err)
	}

	return agentAddr, auth, account, requesterKey, nil
}

func commonGenericAccountSetup(ctx context.Context, from string) (auth *bind.TransactOpts, account accounts.Account, err error) {
	err = checkWalletMigrated()
	if err != nil {
		return nil, accounts.Account{}, err
	}

	as := util.AccountsStore()
	ks := util.KeyStore()
	backends := []accounts.Backend{}
	backends = append(backends, ks)
	manager := accounts.NewManager(&accounts.Config{InsecureUnlockAllowed: false}, backends...)

	var fromAddress common.Address
	if strings.HasPrefix(from, "0x") {
		fromAddress = common.HexToAddress(from)
	} else {
		fromAddress, _, err = as.GetAddrs(strings.ToLower(from))
		if err != nil {
			var e *util.ErrKeyNotFound
			if errors.As(err, &e) {
				return nil, accounts.Account{}, fmt.Errorf("account \"%s\" not found in wallet. Setup with: glif wallet create-account %s", from, from)
			}
			return nil, accounts.Account{}, err
		}
	}

	account = accounts.Account{Address: fromAddress}
	wallet, err := manager.Find(account)
	if err != nil {
		return nil, accounts.Account{}, err
	}

	var passphrase string
	var envSet bool
	var message string
	passphrase, envSet = os.LookupEnv("GLIF_PASSPHRASE")
	message = "Passphrase for account"
	if !envSet {
		err = ks.Unlock(account, "")
		if err != nil {
			prompt := &survey.Password{Message: message}
			survey.AskOne(prompt, &passphrase)
			if passphrase == "" {
				return nil, accounts.Account{}, fmt.Errorf("Aborted")
			}
		}
	}

	auth, err = walletutils.NewEthWalletTransactor(wallet, &account, passphrase, big.NewInt(chainID))
	if err != nil {
		logFatal(err)
	}

	return auth, account, nil
}

func getRequesterKey(as *util.AccountsStorage, ks *keystore.KeyStore) (*ecdsa.PrivateKey, error) {
	requesterAddr, _, err := as.GetAddrs(string(util.RequestKey))
	if err != nil {
		return nil, err
	}
	requesterAccount := accounts.Account{Address: requesterAddr}
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

	return AddressOrAccountNameToEVM(cmd.Context(), agentAddrStr)
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
	as := util.AccountsStore()
	ksLegacy := util.KeyStoreLegacy()

	notMigratedError := fmt.Errorf("wallet not migrated to encrypted keystore. Please run: glif wallet migrate")

	keys := []util.KeyType{
		util.OwnerKey,
		util.OperatorKey,
		util.RequestKey,
	}

	for _, key := range keys {
		_, _, err := as.GetAddrs(string(key))
		if err != nil {
			var e *util.ErrKeyNotFound
			if errors.As(err, &e) {
				_, _, err := ksLegacy.GetAddrs(key)
				if err != nil {
					var e *util.ErrKeyNotFound
					if errors.As(err, &e) {
						// Account not created yet
						continue
					}
					return err
				}
				return notMigratedError
			}
			return err
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

func isFunded(ctx context.Context, caller address.Address) (bool, error) {
	lapi, closer, err := PoolsSDK.Extern().ConnectLotusClient()
	if err != nil {
		return false, err
	}
	defer closer()

	bal, err := lapi.WalletBalance(ctx, caller)
	if err != nil {
		return false, err
	}
	return bal.Cmp(big.NewInt(0)) > 0, nil
}

func confirmBackupExists() error {
	backupExists := os.Getenv("GLIF_BACKUP_EXISTS")
	if backupExists != "" {
		return nil
	}

	as := util.AccountsStore()
	if len(as.AccountNames()) == 0 {
		return nil
	}

	bs := util.BackupsStore()
	confirmedExists, err := bs.Get("confirmed-exists")
	if err != nil {
		return err
	}

	if confirmedExists == "true" {
		return nil
	}

	options := []string{
		"Yes, I made a backup",
		"No, I did not make a backup, abort",
		"No, I did not make a backup, continue anyways (dangerous!!!)",
		"How do I make a backup?",
	}

	choice := ""
	prompt := &survey.Select{
		Message: fmt.Sprintf("The keystore has been updated, have you made a backup of %s ?", cfgDir),
		Options: options,
	}
	survey.AskOne(prompt, &choice)

	if choice == options[0] { // Yes, I made a backup
		bs.Set("confirmed-exists", "true")
		v, _ := time.Now().UTC().MarshalText()
		bs.Set("confirmed-at", string(v))
		color.Green("Keys secured.\n\n")
		return nil
	}

	if choice == options[1] { // No, abort
		return fmt.Errorf("aborting")
	}

	if choice == options[2] { // No, continue
		color.Red("DANGER! Continuing without backup. Make a backup soon!\n\n")
		return nil
	}

	if choice == options[3] { // How do I make a backup?
		fmt.Println("How to make a backup")
		fmt.Println("====================")
		fmt.Println()
		fmt.Println("The configuration and keys are stored in the following directory:")
		fmt.Println()
		fmt.Printf("  %s\n\n", cfgDir)
		fmt.Println("You can use a tool such as zip or tar to make an archive, and")
		fmt.Println("then copy that file to a safe place. In the event of data loss,")
		fmt.Println("you can use the backup to restore the files.")
		fmt.Println()
		fmt.Println("If you lose your keys and you don't have a backup, then you will")
		fmt.Println("lose access to the funds in your agent and control of your miners!")
		Exit(0)
	}

	return nil
}

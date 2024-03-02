package util

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/console/prompt"
	"github.com/ethereum/go-ethereum/params"
)

var AppData = filepath.Join(GetUserHomeDir(), ".struck")

var Shutdown = false

func GetUserHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return h
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func PathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	} else {
		return true
	}
}

func ToPubKey(key string) (phase0.BLSPubKey, error) {
	pkey, _ := hexutil.Decode(key)
	k := phase0.BLSPubKey{}
	x := copy(k[:], pkey[:])
	if x != 48 {
		return phase0.BLSPubKey{}, fmt.Errorf("bad copy of public key")
	} else {
		return k, nil
	}
}
func WrapError(err error, msg string, params ...any) error {
	emsg := fmt.Sprintf(msg, params...)
	return fmt.Errorf("%s:%v", emsg, err)
}
func EtherToWei(val *big.Int) *big.Int {
	return new(big.Int).Mul(val, big.NewInt(params.Ether))
}

func WeiToEther(val *big.Int) *big.Int {
	return new(big.Int).Div(val, big.NewInt(params.Ether))
}

func GetPassPhrase(confirmation bool) (*string, error) {
	password, err := prompt.Stdin.PromptPassword("Password: ")
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %v", err)
	}
	if confirmation {
		confirm, err := prompt.Stdin.PromptPassword("Repeat password: ")
		if err != nil {
			return nil, fmt.Errorf("failed to read password confirmation: %v", err)
		}
		if password != confirm {
			return nil, fmt.Errorf("passwords do not match")
		}
	}
	return &password, nil
}

package accounts

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/allisterb/strac/blockchain"
	"github.com/allisterb/strac/util"
)

var log = logging.Logger("strac/accounts")

func NewAccount(WalletDir *string) error {
	if WalletDir != nil {
		log.Infof("Creating keystore file at %s...", *WalletDir)
		log.Info("Enter the passphrase for this keystore file")
		_, err := util.GetPassPhrase(true)
		if err != nil {
			return err
		}
		return fmt.Errorf("not implemented")
	}
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	privateKeyBytes := crypto.FromECDSA(privateKey)
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	log.Infof("New Stratis account address: %v", address.Hex())
	log.Infof("New Stratis account public key: %v", hexutil.Encode(publicKeyBytes)[4:])
	log.Infof("New Stratis account private key: %v", hexutil.Encode(privateKeyBytes)[2:])
	log.Warnf("Make sure you record the private key of every account you wish to use...there is no way to recover a private key for an account if you lose it. The above is sensitive info and should be stored privately and securely.")
	return nil
}

func BalanceAt(_account string, _block int64) error {
	bytes, err := hexutil.Decode(_account)
	if err != nil {
		return err
	}
	var block *big.Int = nil
	if _block != 0 {
		block = big.NewInt(_block)
	}
	account := common.BytesToAddress(bytes)
	bal, err := blockchain.ExecutionClient.BalanceAt(blockchain.Ctx, account, block)
	if err != nil {
		return err
	} else {
		log.Infof("Balance of account %v is %v STRAX.", account, util.WeiToEther(bal))
		return nil
	}
}

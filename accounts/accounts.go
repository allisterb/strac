package accounts

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("strick/accounts")

func Create() error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	privateKeyBytes := crypto.FromECDSA(privateKey)
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	log.Infof("New Stratis account address: %v", address)
	log.Infof("New Stratis account private key: %v", hexutil.Encode(privateKeyBytes)[2:])
	log.Debugf("New Stratis account public key: %v", hexutil.Encode(publicKeyBytes)[4:])
	//hash := sha3.NewLegacyKeccak256()
	//hash.Write(publicKeyBytes[1:])
	//fmt.Println(hexutil.Encode(hash.Sum(nil)[12:]))
	return nil
}

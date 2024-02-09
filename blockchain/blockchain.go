package blockchain

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("strick/blockchain")

func Ping(ctx context.Context, rpcurl string) error {
	client, err := ethclient.Dial(rpcurl)
	if err != nil {
		return err
	}
	chainid, err := client.ChainID(ctx)
	if err != nil {
		return err
	} else {
		log.Infof("Chain id of node at %v is %v.", rpcurl, chainid)
		return nil
	}

}

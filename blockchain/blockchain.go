package blockchain

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("strick/blockchain")

var HttpUrl = ""

func Ping(ctx context.Context) error {
	client, err := ethclient.DialContext(ctx, HttpUrl)
	if err != nil {
		return err
	}
	chainid, err := client.ChainID(ctx)
	if err != nil {
		return err
	} else {
		log.Infof("Chain id of node at %v is %v.", HttpUrl, chainid)
	}
	sp, err := client.SyncProgress(ctx)
	if err != nil {
		return err
	} else if sp == nil {
		log.Warnf("Could not get sync progress of node at %v.", HttpUrl)
	} else {
		log.Infof("Node at %v is at block %v of %v. Node synced: %v.", HttpUrl, sp.CurrentBlock, sp.HighestBlock, sp.Done())
	}
	return nil

}

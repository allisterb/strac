package blockchain

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("strick/blockchain")

var HttpUrl = ""
var Client *ethclient.Client
var Ctx context.Context

func Init(httpUrl string, timeout int) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second) //nolint:all
	client, err := ethclient.DialContext(ctx, httpUrl)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		HttpUrl = httpUrl
		Client = client
		Ctx = ctx
		return nil
	}
}

func Ping() error {
	chainid, err := Client.ChainID(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		log.Infof("Chain id of node at %v is %v.", HttpUrl, chainid)
	}
	block, err := Client.BlockNumber(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		log.Infof("Most recent block of node at %v is %v.", HttpUrl, block)
	}
	sp, err := Client.SyncProgress(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else if sp == nil {
		log.Warnf("Could not get sync progress of node at %v.", HttpUrl)
	} else {
		log.Infof("Node at %v is at block %v of %v. Node synced: %v.", HttpUrl, sp.CurrentBlock, sp.HighestBlock, sp.Done())
	}

	return nil
}

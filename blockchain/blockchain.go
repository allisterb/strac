package blockchain

import (
	"context"
	"fmt"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	"github.com/attestantio/go-eth2-client/api"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"
	"github.com/rs/zerolog"
)

var log = logging.Logger("strick/blockchain")

var HttpUrl = ""
var BeaconHttpUrl = ""
var NodeClient *ethclient.Client
var BeaconClient eth2client.Service
var Ctx context.Context

func Init(httpUrl string, beaconHttpUrl string, timeout int) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second) //nolint:all
	client, err := ethclient.DialContext(ctx, httpUrl)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	}
	bclient, err := http.New(ctx,
		// WithAddress supplies the address of the beacon node, as a URL.
		http.WithAddress(beaconHttpUrl),
		// LogLevel supplies the level of logging to carry out.
		http.WithLogLevel(zerolog.ErrorLevel),
	)
	if err != nil {
		return err
	}
	HttpUrl = httpUrl
	BeaconHttpUrl = beaconHttpUrl
	NodeClient = client
	BeaconClient = bclient
	Ctx = ctx
	return nil
}

func Ping() error {
	chainid, err := NodeClient.ChainID(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		log.Infof("Chain id of node at %v is %v.", HttpUrl, chainid)
	}
	block, err := NodeClient.BlockNumber(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		log.Infof("Most recent block of node at %v is %v.", HttpUrl, block)
	}
	sp, err := NodeClient.SyncProgress(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else if sp == nil {
		log.Warnf("Could not get sync progress of node at %v.", HttpUrl)
	} else {
		log.Infof("Node at %v is at block %v of %v. Node synced: %v.", HttpUrl, sp.CurrentBlock, sp.HighestBlock, sp.Done())
	}
	return nil
}

func Info(genesis bool, validators bool) error {
	if genesis {
		if provider, isProvider := BeaconClient.(eth2client.GenesisProvider); isProvider {
			response, err := provider.Genesis(Ctx, &api.GenesisOpts{})
			if err != nil {
				return err
			} else {
				log.Infof("Genesis time: %v", response.Data.GenesisTime)
				log.Infof("Genesis validator root: %v", response.Data.GenesisValidatorsRoot.String)
				log.Infof("Genesis fork current version: %v", hexutil.Encode(response.Data.GenesisForkVersion[:]))
			}
		} else {
			return fmt.Errorf("could not get GenesisProvider interface")
		}
		if provider, isProvider := BeaconClient.(eth2client.ForkProvider); isProvider {
			response, err := provider.Fork(Ctx, &api.ForkOpts{State: "head"})
			if err != nil {
				return err
			} else {
				log.Infof("Genesis fork previous version: %v", hexutil.Encode(response.Data.PreviousVersion[:]))

			}
		} else {
			return fmt.Errorf("could not get ForkProvider interface")
		}
	}

	if validators {
		if provider, isProvider := BeaconClient.(eth2client.ValidatorsProvider); isProvider {
			response, err := provider.Validators(Ctx, &api.ValidatorsOpts{State: "head"})
			if err != nil {
				return err
			} else {
				log.Infof("validator: %v", response.Data)

			}
		} else {
			return fmt.Errorf("could not get validator interface")
		}
	}
	return nil
}

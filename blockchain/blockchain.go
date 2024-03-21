package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	"github.com/attestantio/go-eth2-client/api"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"
	"github.com/rs/zerolog"
)

var log = logging.Logger("strac/blockchain")

var HttpUrl = ""
var BeaconHttpUrl = ""
var ExecutionClient *ethclient.Client
var BeaconClient eth2client.Service
var Ctx context.Context

func InitEC(httpUrl string) error {
	client, err := ethclient.DialContext(Ctx, httpUrl)
	if err != nil {
		return fmt.Errorf("error connecting to node: %v", err)
	}
	HttpUrl = httpUrl
	ExecutionClient = client
	return nil
}

func InitCC(beaconHttpUrl string, timeout int) error {
	if BeaconClient != nil {
		return nil
	}
	bclient, err := http.New(Ctx,
		// WithAddress supplies the address of the beacon node, as a URL.
		http.WithAddress(beaconHttpUrl),
		// LogLevel supplies the level of logging to carry out.
		http.WithLogLevel(zerolog.Disabled),
		http.WithTimeout(time.Duration(timeout)*time.Second),
	)
	if err != nil {
		return err
	}
	BeaconHttpUrl = beaconHttpUrl
	BeaconClient = bclient
	return nil
}

func GetChainID() (*big.Int, error) {
	return ExecutionClient.ChainID(Ctx)
}
func Ping() error {
	chainid, err := ExecutionClient.ChainID(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		log.Infof("Chain id of node at %v is %v.", HttpUrl, chainid)
	}
	block, err := ExecutionClient.BlockNumber(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else {
		log.Infof("Most recent block of node at %v is %v.", HttpUrl, block)
	}
	sp, err := ExecutionClient.SyncProgress(Ctx)
	if err != nil {
		return fmt.Errorf("error pinging node: %v", err)
	} else if sp == nil {
		log.Warnf("Could not get sync progress of node at %v.", HttpUrl)
	} else {
		log.Infof("Node at %v is at block %v of %v. Node synced: %v.", HttpUrl, sp.CurrentBlock, sp.HighestBlock, sp.Done())
	}
	return nil
}

func Info(genesis bool, validatorPubKey string, peers bool) error {
	if validatorPubKey != "" {
		if provider, isProvider := BeaconClient.(eth2client.ValidatorsProvider); isProvider {
			pkey, _ := hexutil.Decode(validatorPubKey)
			k := phase0.BLSPubKey{}
			x := copy(k[:], pkey[:])
			if x != 48 {
				return fmt.Errorf("bad copy of validator public key")
			}
			response, err := provider.Validators(Ctx, &api.ValidatorsOpts{PubKeys: []phase0.BLSPubKey{k}, State: "head"})
			if err != nil {
				return err
			} else if len(response.Data) != 1 {
				return fmt.Errorf("length of response data is %v", len(response.Data))
			} else {
				for _, v := range response.Data {
					log.Infof("Validator index: %v", v.Index)
					log.Infof("Validator public key: %v", hexutil.Encode(v.Validator.PublicKey[:]))
					log.Infof("Validator activation eligibility epoch: %v", v.Validator.ActivationEligibilityEpoch)
					log.Infof("Validator activation epoch: %v", v.Validator.ActivationEpoch)
					log.Infof("Validator effective balance: %v", v.Validator.EffectiveBalance/1000000000)
					log.Infof("Validator withdrawal credentials: %v", hexutil.Encode(v.Validator.WithdrawalCredentials))
				}
			}
		} else {
			return fmt.Errorf("could not get validator interface")
		}
		return nil
	}

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

	if peers {
		if provider, isProvider := BeaconClient.(eth2client.NodePeersProvider); isProvider {
			response, err := provider.NodePeers(Ctx, &api.NodePeersOpts{State: []string{"connected"}})
			if err != nil {
				return err
			} else {
				inbound := 0
				outbound := 0
				for _, p := range response.Data {
					log.Infof("Peer id: %v", p.PeerID)
					log.Infof("Peer last seen p2p address: %v", p.LastSeenP2PAddress)
					log.Infof("Peer state: %v", p.State)
					log.Infof("Peer direction: %v\n", p.Direction)
					if p.Direction == "inbound" {
						inbound++
					} else {
						outbound++
					}
				}
				log.Infof("Inbound peers: %v", inbound)
				log.Infof("Outbound peers: %v", outbound)
				log.Infof("Total connected peers: %v", inbound+outbound)
			}
		} else {
			return fmt.Errorf("could not get GenesisProvider interface")
		}

	}

	return nil
}

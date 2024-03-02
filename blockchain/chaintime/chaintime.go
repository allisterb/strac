package chaintime

import (
	"fmt"
	"strconv"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	"github.com/attestantio/go-eth2-client/api"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/allisterb/strac/blockchain"
	"github.com/allisterb/strac/util"
)

// ChainTime provides chain time ChainTimes.
type ChainTime struct {
	genesisTime                  time.Time
	slotDuration                 time.Duration
	slotsPerEpoch                uint64
	epochsPerSyncCommitteePeriod uint64
}

type parameters struct {
	logLevel        logging.LogLevel
	genesisProvider eth2client.GenesisProvider
	specProvider    eth2client.SpecProvider
}

// Parameter is the interface for ChainTime parameters.
type Parameter interface {
	apply(p *parameters)
}

type parameterFunc func(*parameters)

var log = logging.Logger("strac/blockchain/chaintime")

func (f parameterFunc) apply(p *parameters) {
	f(p)
}

// WithLogLevel sets the log level for the module.
func WithLogLevel(logLevel logging.LogLevel) Parameter {
	return parameterFunc(func(p *parameters) {
		p.logLevel = logLevel
	})
}

// WithGenesisProvider sets the genesis time provider.
func WithGenesisProvider(provider eth2client.GenesisProvider) Parameter {
	return parameterFunc(func(p *parameters) {
		p.genesisProvider = provider
	})
}

// WithSpecProvider sets the spec provider.
func WithSpecProvider(provider eth2client.SpecProvider) Parameter {
	return parameterFunc(func(p *parameters) {
		p.specProvider = provider
	})
}

// parseAndCheckParameters parses and checks parameters to ensure that mandatory parameters are present and correct.
func parseAndCheckParameters(params ...Parameter) (*parameters, error) {
	parameters := parameters{
		logLevel: logging.LevelInfo,
	}
	for _, p := range params {
		if params != nil {
			p.apply(&parameters)
		}
	}

	if parameters.specProvider == nil {
		return nil, fmt.Errorf("no spec provider specified")
	}
	if parameters.genesisProvider == nil {
		return nil, fmt.Errorf("no genesis provider specified")
	}

	return &parameters, nil
}

// New creates a new controller.
func NewChainTime(params ...Parameter) (*ChainTime, error) {
	parameters, err := parseAndCheckParameters(params...)
	if err != nil {
		return nil, util.WrapError(err, "problem with parameters")
	}

	genesisResponse, err := parameters.genesisProvider.Genesis(blockchain.Ctx, &api.GenesisOpts{})
	if err != nil {
		return nil, util.WrapError(err, "failed to obtain genesis time")
	}
	log.Debugf("Genesis time: %v", genesisResponse.Data.GenesisTime)

	specResponse, err := parameters.specProvider.Spec(blockchain.Ctx, &api.SpecOpts{})
	if err != nil {
		return nil, util.WrapError(err, "failed to obtain spec")
	}

	tmp, exists := specResponse.Data["SECONDS_PER_SLOT"]
	if !exists {
		return nil, fmt.Errorf("SECONDS_PER_SLOT not found in spec")
	}
	slotDuration, ok := tmp.(time.Duration)
	if !ok {
		return nil, fmt.Errorf("SECONDS_PER_SLOT of unexpected type")
	}

	tmp, exists = specResponse.Data["SLOTS_PER_EPOCH"]
	if !exists {
		return nil, fmt.Errorf("SLOTS_PER_EPOCH not found in spec")
	}
	slotsPerEpoch, ok := tmp.(uint64)
	if !ok {
		return nil, fmt.Errorf("SLOTS_PER_EPOCH of unexpected type")
	}

	var epochsPerSyncCommitteePeriod uint64
	if tmp, exists := specResponse.Data["EPOCHS_PER_SYNC_COMMITTEE_PERIOD"]; exists {
		tmp2, ok := tmp.(uint64)
		if !ok {
			return nil, fmt.Errorf("EPOCHS_PER_SYNC_COMMITTEE_PERIOD of unexpected type")
		}
		epochsPerSyncCommitteePeriod = tmp2
	}

	s := &ChainTime{
		genesisTime:                  genesisResponse.Data.GenesisTime,
		slotDuration:                 slotDuration,
		slotsPerEpoch:                slotsPerEpoch,
		epochsPerSyncCommitteePeriod: epochsPerSyncCommitteePeriod,
	}

	return s, nil
}

// GenesisTime provides the time of the chain's genesis.
func (s *ChainTime) GenesisTime() time.Time {
	return s.genesisTime
}

// SlotsPerEpoch provides the number of slots in the chain's epoch.
func (s *ChainTime) SlotsPerEpoch() uint64 {
	return s.slotsPerEpoch
}

// SlotDuration provides the duration of the chain's slot.
func (s *ChainTime) SlotDuration() time.Duration {
	return s.slotDuration
}

// StartOfSlot provides the time at which a given slot starts.
func (s *ChainTime) StartOfSlot(slot phase0.Slot) time.Time {
	return s.genesisTime.Add(time.Duration(slot) * s.slotDuration)
}

// StartOfEpoch provides the time at which a given epoch starts.
func (s *ChainTime) StartOfEpoch(epoch phase0.Epoch) time.Time {
	return s.genesisTime.Add(time.Duration(uint64(epoch)*s.slotsPerEpoch) * s.slotDuration)
}

// CurrentSlot provides the current slot.
func (s *ChainTime) CurrentSlot() phase0.Slot {
	if s.genesisTime.After(time.Now()) {
		return 0
	}
	return phase0.Slot(uint64(time.Since(s.genesisTime).Seconds()) / uint64(s.slotDuration.Seconds()))
}

// CurrentEpoch provides the current epoch.
func (s *ChainTime) CurrentEpoch() phase0.Epoch {
	return phase0.Epoch(uint64(s.CurrentSlot()) / s.slotsPerEpoch)
}

// CurrentSyncCommitteePeriod provides the current sync committee period.
func (s *ChainTime) CurrentSyncCommitteePeriod() uint64 {
	return uint64(s.CurrentEpoch()) / s.epochsPerSyncCommitteePeriod
}

// SlotToEpoch provides the epoch of a given slot.
func (s *ChainTime) SlotToEpoch(slot phase0.Slot) phase0.Epoch {
	return phase0.Epoch(uint64(slot) / s.slotsPerEpoch)
}

// SlotToSyncCommitteePeriod provides the sync committee period of the given slot.
func (s *ChainTime) SlotToSyncCommitteePeriod(slot phase0.Slot) uint64 {
	return uint64(s.SlotToEpoch(slot)) / s.epochsPerSyncCommitteePeriod
}

// FirstSlotOfEpoch provides the first slot of the given epoch.
func (s *ChainTime) FirstSlotOfEpoch(epoch phase0.Epoch) phase0.Slot {
	return phase0.Slot(uint64(epoch) * s.slotsPerEpoch)
}

// LastSlotOfEpoch provides the last slot of the given epoch.
func (s *ChainTime) LastSlotOfEpoch(epoch phase0.Epoch) phase0.Slot {
	return phase0.Slot(uint64(epoch)*s.slotsPerEpoch + s.slotsPerEpoch - 1)
}

// TimestampToSlot provides the slot of the given timestamp.
func (s *ChainTime) TimestampToSlot(timestamp time.Time) phase0.Slot {
	if timestamp.Before(s.genesisTime) {
		return 0
	}
	secondsSinceGenesis := uint64(timestamp.Sub(s.genesisTime).Seconds())
	return phase0.Slot(secondsSinceGenesis / uint64(s.slotDuration.Seconds()))
}

// TimestampToEpoch provides the epoch of the given timestamp.
func (s *ChainTime) TimestampToEpoch(timestamp time.Time) phase0.Epoch {
	if timestamp.Before(s.genesisTime) {
		return 0
	}
	secondsSinceGenesis := uint64(timestamp.Sub(s.genesisTime).Seconds())
	return phase0.Epoch(secondsSinceGenesis / uint64(s.slotDuration.Seconds()) / s.slotsPerEpoch)
}

// ParseEpoch parses input to calculate the desired epoch.
func ParseEpoch(chainTime *ChainTime, epochStr string) (phase0.Epoch, error) {
	currentEpoch := chainTime.CurrentEpoch()
	switch epochStr {
	case "", "current", "head", "-0":
		return currentEpoch, nil
	case "last":
		if currentEpoch > 0 {
			currentEpoch--
		}
		return currentEpoch, nil
	default:
		val, err := strconv.ParseInt(epochStr, 10, 64)
		if err != nil {
			return 0, errors.Wrap(err, "failed to parse epoch")
		}
		if val >= 0 {
			return phase0.Epoch(val), nil
		}
		if phase0.Epoch(-val) > currentEpoch {
			return 0, nil
		}
		return currentEpoch + phase0.Epoch(val), nil
	}
}

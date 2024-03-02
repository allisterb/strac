package validators

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	eth2client "github.com/attestantio/go-eth2-client"
	api "github.com/attestantio/go-eth2-client/api"
	apiv1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/pkg/errors"

	"github.com/allisterb/strac/blockchain"
	"github.com/allisterb/strac/blockchain/chaintime"
	"github.com/allisterb/strac/util"
)

type validatorFault struct {
	Validator         phase0.ValidatorIndex   `json:"validator_index"`
	AttestationData   *phase0.AttestationData `json:"attestation_data,omitempty"`
	InclusionDistance int                     `json:"inclusion_delay"`
}

type nonParticipatingValidator struct {
	Validator phase0.ValidatorIndex `json:"validator_index"`
	Slot      phase0.Slot           `json:"slot"`
	Committee phase0.CommitteeIndex `json:"committee_index"`
}

type slot struct {
	Slot         phase0.Slot       `json:"slot"`
	Attestations *slotAttestations `json:"attestations"`
}

type slotAttestations struct {
	Expected      int `json:"expected"`
	Included      int `json:"included"`
	CorrectHead   int `json:"correct_head"`
	TimelyHead    int `json:"timely_head"`
	CorrectTarget int `json:"correct_target"`
	TimelyTarget  int `json:"timely_target"`
	TimelySource  int `json:"timely_source"`
}

type epochProposal struct {
	Slot     phase0.Slot           `json:"slot"`
	Proposer phase0.ValidatorIndex `json:"proposer"`
	Block    bool                  `json:"block"`
}

type epochSyncCommittee struct {
	Index  phase0.ValidatorIndex `json:"index"`
	Missed int                   `json:"missed"`
}

type validatorSummary struct {
	Epoch                      phase0.Epoch                 `json:"epoch"`
	Validators                 []*apiv1.Validator           `json:"validators"`
	FirstSlot                  phase0.Slot                  `json:"first_slot"`
	LastSlot                   phase0.Slot                  `json:"last_slot"`
	ActiveValidators           int                          `json:"active_validators"`
	ParticipatingValidators    int                          `json:"participating_validators"`
	NonParticipatingValidators []*nonParticipatingValidator `json:"non_participating_validators"`
	IncorrectHeadValidators    []*validatorFault            `json:"incorrect_head_validators"`
	UntimelyHeadValidators     []*validatorFault            `json:"untimely_head_validators"`
	UntimelySourceValidators   []*validatorFault            `json:"untimely_source_validators"`
	IncorrectTargetValidators  []*validatorFault            `json:"incorrect_target_validators"`
	UntimelyTargetValidators   []*validatorFault            `json:"untimely_target_validators"`
	Slots                      []*slot                      `json:"slots"`
	Proposals                  []*epochProposal             `json:"-"`
	SyncCommittee              []*epochSyncCommittee        `json:"-"`
}

var validatorsProvider eth2client.ValidatorsProvider
var genesisProvider eth2client.GenesisProvider
var specProvider eth2client.SpecProvider
var pdProvider eth2client.ProposerDutiesProvider
var blocksProvider eth2client.SignedBeaconBlockProvider
var chainTime *chaintime.ChainTime

func Init() error {
	isProvider := false
	var err error
	validatorsProvider, isProvider = blockchain.BeaconClient.(eth2client.ValidatorsProvider)
	if !isProvider {
		return fmt.Errorf("could not get validator interface")
	}
	genesisProvider, isProvider = blockchain.BeaconClient.(eth2client.GenesisProvider)
	if !isProvider {
		return fmt.Errorf("could not get genesis interface")
	}
	specProvider, isProvider = blockchain.BeaconClient.(eth2client.SpecProvider)
	if !isProvider {
		return fmt.Errorf("could not get spec interface")
	}
	pdProvider, isProvider = blockchain.BeaconClient.(eth2client.ProposerDutiesProvider)
	if !isProvider {
		return fmt.Errorf("could not get proposer duties interface")
	}
	blocksProvider, isProvider = blockchain.BeaconClient.(eth2client.SignedBeaconBlockProvider)
	if !isProvider {
		return fmt.Errorf("could not get signed beacon block interface")
	}
	chainTime, err = chaintime.NewChainTime(chaintime.WithGenesisProvider(genesisProvider), chaintime.WithSpecProvider(specProvider))
	if err != nil {
		return util.WrapError(err, "could not get chain time")
	}
	return nil
}

func Summary(validatorsStr []string, stateID string, epoch string) (*validatorSummary, error) {
	var err error

	summary := &validatorSummary{}
	summary.Epoch, err = chaintime.ParseEpoch(chainTime, epoch)
	if err != nil {
		return nil, util.WrapError(err, "failed to parse epoch")
	}
	summary.FirstSlot = chainTime.FirstSlotOfEpoch(summary.Epoch)
	summary.LastSlot = chainTime.FirstSlotOfEpoch(summary.Epoch+1) - 1
	summary.Slots = make([]*slot, 1+int(summary.LastSlot)-int(summary.FirstSlot))
	for i := range summary.Slots {
		summary.Slots[i] = &slot{
			Slot: summary.FirstSlot + phase0.Slot(i),
		}
	}

	summary.Validators, err = parseValidators(blockchain.Ctx, validatorsStr, fmt.Sprintf("%d", summary.FirstSlot))
	if err != nil {
		return nil, err
	}
	sort.Slice(summary.Validators, func(i int, j int) bool {
		return summary.Validators[i].Index < summary.Validators[j].Index
	})

	// Create a map for validator indices for easy lookup.
	validatorsByIndex := make(map[phase0.ValidatorIndex]*apiv1.Validator)
	for _, validator := range summary.Validators {
		validatorsByIndex[validator.Index] = validator
	}

	if err := processProposerDuties(validatorsByIndex, *summary); err != nil {
		return nil, err
	}

	return summary, nil
}

// ParseValidators parses input to obtain the list of validators.
func parseValidators(ctx context.Context, validatorsStr []string, stateID string) ([]*apiv1.Validator, error) {
	validators := make([]*apiv1.Validator, 0, len(validatorsStr))
	indices := make([]phase0.ValidatorIndex, 0)
	for i := range validatorsStr {
		if strings.Contains(validatorsStr[i], "-") {
			// Range.
			bits := strings.Split(validatorsStr[i], "-")
			if len(bits) != 2 {
				return nil, fmt.Errorf("invalid range %s", validatorsStr[i])
			}
			low, err := strconv.ParseUint(bits[0], 10, 64)
			if err != nil {
				return nil, util.WrapError(err, "invalid range start")
			}
			high, err := strconv.ParseUint(bits[1], 10, 64)
			if err != nil {
				return nil, util.WrapError(err, "invalid range end")
			}
			for index := low; index <= high; index++ {
				indices = append(indices, phase0.ValidatorIndex(index))
			}
		} else {
			index, err := strconv.ParseUint(validatorsStr[i], 10, 64)
			if err != nil {
				return nil, util.WrapError(err, "failed to parse validator %s", validatorsStr[i])
			}
			indices = append(indices, phase0.ValidatorIndex(index))
		}
	}

	response, err := validatorsProvider.Validators(ctx, &api.ValidatorsOpts{State: stateID, Indices: indices})
	if err != nil {
		return nil, util.WrapError(err, fmt.Sprintf("failed to obtain validators %v", indices))
	}
	for _, validator := range response.Data {
		validators = append(validators, validator)
	}
	return validators, nil
}

// ParseValidator parses input to obtain the validator.
func parseValidator(ctx context.Context,
	validatorsProvider eth2client.ValidatorsProvider,
	validatorStr string,
	stateID string,
) (
	*apiv1.Validator,
	error,
) {
	var validators map[phase0.ValidatorIndex]*apiv1.Validator

	// Could be a simple index.
	index, err := strconv.ParseUint(validatorStr, 10, 64)
	if err == nil {
		response, err := validatorsProvider.Validators(ctx, &api.ValidatorsOpts{
			State:   stateID,
			Indices: []phase0.ValidatorIndex{phase0.ValidatorIndex(index)},
		})
		if err != nil {
			return nil, util.WrapError(err, "failed to obtain validator information")
		}
		validators = response.Data
	} else {
		pubKey, err := util.ToPubKey(validatorStr)
		if err != nil {
			return nil, util.WrapError(err, "unable to obtain public key for account")
		}
		validatorsResponse, err := validatorsProvider.Validators(ctx, &api.ValidatorsOpts{
			State:   stateID,
			PubKeys: []phase0.BLSPubKey{pubKey},
		})
		if err != nil {
			return nil, util.WrapError(err, "failed to obtain validator information")
		}
		validators = validatorsResponse.Data
	}

	// Validator is first and only entry in the map.
	for _, validator := range validators {
		return validator, nil
	}

	return nil, fmt.Errorf("unknown validator")
}

func processProposerDuties(validatorsByIndex map[phase0.ValidatorIndex]*apiv1.Validator, summary validatorSummary) error {
	response, err := pdProvider.ProposerDuties(blockchain.Ctx, &api.ProposerDutiesOpts{
		Epoch: summary.Epoch,
	})
	if err != nil {
		return util.WrapError(err, "failed to obtain proposer duties")
	}
	for _, duty := range response.Data {
		if _, exists := validatorsByIndex[duty.ValidatorIndex]; !exists {
			continue
		}
		blockResponse, err := blocksProvider.SignedBeaconBlock(blockchain.Ctx, &api.SignedBeaconBlockOpts{
			Block: fmt.Sprintf("%d", duty.Slot),
		})
		if err != nil {
			var apiErr *api.Error
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				return nil
			}

			return errors.Wrap(err, fmt.Sprintf("failed to obtain block for slot %d", duty.Slot))
		}
		block := blockResponse.Data
		present := block != nil
		summary.Proposals = append(summary.Proposals, &epochProposal{
			Slot:     duty.Slot,
			Proposer: duty.ValidatorIndex,
			Block:    present,
		})
	}

	return nil
}

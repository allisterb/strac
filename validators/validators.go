package validators

import (
	"bytes"
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
	logging "github.com/ipfs/go-log/v2"
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
var beaconBlockHeadersProvider eth2client.BeaconBlockHeadersProvider
var attesterDutiesProvider eth2client.AttesterDutiesProvider
var chainTime *chaintime.ChainTime

var log = logging.Logger("strac/validators")

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

	beaconBlockHeadersProvider, isProvider = blockchain.BeaconClient.(eth2client.BeaconBlockHeadersProvider)
	if !isProvider {
		return fmt.Errorf("could not get beacon block headers interface")
	}

	attesterDutiesProvider, isProvider = blockchain.BeaconClient.(eth2client.AttesterDutiesProvider)
	if !isProvider {
		return fmt.Errorf("could not get attester duties provider interface")
	}

	chainTime, err = chaintime.NewChainTime(chaintime.WithGenesisProvider(genesisProvider), chaintime.WithSpecProvider(specProvider))
	if err != nil {
		return util.WrapError(err, "could not get chain time")
	}

	return nil
}
func Summary(validatorsStr []string, stateID string, start string, end string, numEpochs string) error {
	var err error
	var startEpoch phase0.Epoch
	var endEpoch phase0.Epoch
	var n uint64

	if start == "" && end == "" && numEpochs == "" {
		return fmt.Errorf("at least one of start or end or numEpochs must be specified")
	}
	if start != "" && end != "" && numEpochs != "" {
		return fmt.Errorf("you can't specify all 3 of start and end and numEpochs")
	}
	if err = Init(); err != nil {
		return err
	}

	if start != "" && numEpochs != "" {
		if startEpoch, err = chaintime.ParseEpoch(chainTime, start); err != nil {
			return err
		}
		if n, err = strconv.ParseUint(numEpochs, 10, 0); err != nil {
			return err
		}
		endEpoch = startEpoch + phase0.Epoch(n)
	} else if end != "" && numEpochs != "" {
		if endEpoch, err = chaintime.ParseEpoch(chainTime, end); err != nil {
			return err
		}
		if n, err = strconv.ParseUint(numEpochs, 10, 0); err != nil {
			return err
		}
		startEpoch = endEpoch - phase0.Epoch(n)
	} else if start != "" && end != "" {
		if startEpoch, err = chaintime.ParseEpoch(chainTime, start); err != nil {
			return err
		}
		if endEpoch, err = chaintime.ParseEpoch(chainTime, end); err != nil {
			return err
		}
	}

	log.Infof("start epoch: %v, end epoch: %v", startEpoch, endEpoch)
	if startEpoch > endEpoch {
		return fmt.Errorf("the start epoch specified: %v is greater than the end epoch specifed: %v", startEpoch, endEpoch)
	}

	/*
		if err != nil {
			return err
		}



		if endEpoch == 0 {
			return nil
		}

		if err := Init(); err != nil {
			return err
		}
	*/
	return nil
}

func EpochSummary(validatorsStr []string, stateID string, epoch string) (*validatorSummary, error) {
	var err error
	log.Infof("fetching validator(s) summary for epoch %s...", epoch)
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

	if err := processProposerDuties(validatorsByIndex, summary); err != nil {
		return nil, err
	}

	if err = processAttesterDuties(validatorsByIndex, summary); err != nil {
		return nil, err
	}

	builder := strings.Builder{}

	builder.WriteString("Epoch ")
	builder.WriteString(fmt.Sprintf("%d:\n", summary.Epoch))
	if len(summary.Proposals) > 0 {
		builder.WriteString("  Proposer validators:\n")
		for _, p := range summary.Proposals {
			validator := validatorsByIndex[p.Proposer]
			builder.WriteString(fmt.Sprintf("    %s\n", validator.Validator.PublicKey.String()))
		}
	}
	if len(summary.NonParticipatingValidators) > 0 {
		builder.WriteString("  Non-participating validators:\n")
		for _, validator := range summary.NonParticipatingValidators {
			builder.WriteString(fmt.Sprintf("    %d (slot %d, committee %d)\n", validator.Validator, validator.Slot, validator.Committee))
		}
	}
	if len(summary.IncorrectHeadValidators) > 0 {
		builder.WriteString("  Incorrect head validators:\n")
		for _, validator := range summary.IncorrectHeadValidators {
			builder.WriteString(fmt.Sprintf("    %d (slot %d, committee %d)\n", validator.Validator, validator.AttestationData.Slot, validator.AttestationData.Index))
		}
	}
	if len(summary.UntimelyHeadValidators) > 0 {
		builder.WriteString("  Untimely head validators:\n")
		for _, validator := range summary.UntimelyHeadValidators {
			builder.WriteString(fmt.Sprintf("    %d (slot %d, committee %d, inclusion distance %d)\n", validator.Validator, validator.AttestationData.Slot, validator.AttestationData.Index, validator.InclusionDistance))
		}
	}
	if len(summary.UntimelySourceValidators) > 0 {
		builder.WriteString("  Untimely source validators:\n")
		for _, validator := range summary.UntimelySourceValidators {
			builder.WriteString(fmt.Sprintf("    %d (slot %d, committee %d, inclusion distance %d)\n", validator.Validator, validator.AttestationData.Slot, validator.AttestationData.Index, validator.InclusionDistance))
		}
	}
	if len(summary.IncorrectTargetValidators) > 0 {
		builder.WriteString("  Incorrect target validators:\n")
		for _, validator := range summary.IncorrectTargetValidators {
			builder.WriteString(fmt.Sprintf("    %d (slot %d, committee %d)\n", validator.Validator, validator.AttestationData.Slot, validator.AttestationData.Index))
		}
	}
	if len(summary.UntimelyTargetValidators) > 0 {
		builder.WriteString("  Untimely target validators:\n")
		for _, validator := range summary.UntimelyTargetValidators {
			builder.WriteString(fmt.Sprintf("    %d (slot %d, committee %d, inclusion distance %d)\n", validator.Validator, validator.AttestationData.Slot, validator.AttestationData.Index, validator.InclusionDistance))
		}
	}
	log.Infof("Summary:\n%s", builder.String())

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

func processProposerDuties(validatorsByIndex map[phase0.ValidatorIndex]*apiv1.Validator, summary *validatorSummary) error {
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

func getActiveValidators(validatorsByIndex map[phase0.ValidatorIndex]*apiv1.Validator, summary *validatorSummary) (map[phase0.ValidatorIndex]*apiv1.Validator, []phase0.ValidatorIndex) {
	activeValidators := make(map[phase0.ValidatorIndex]*apiv1.Validator)
	activeValidatorIndices := make([]phase0.ValidatorIndex, 0, len(validatorsByIndex))
	for _, validator := range summary.Validators {
		if validator.Validator.ActivationEpoch <= summary.Epoch && validator.Validator.ExitEpoch > summary.Epoch {
			activeValidators[validator.Index] = validator
			activeValidatorIndices = append(activeValidatorIndices, validator.Index)
		}
	}

	return activeValidators, activeValidatorIndices
}

func processAttesterDuties(validatorsByIndex map[phase0.ValidatorIndex]*apiv1.Validator, summary *validatorSummary) error {
	activeValidators, activeValidatorIndices := getActiveValidators(validatorsByIndex, summary)

	// Obtain number of validators that voted for blocks in the epoch.
	// These votes can be included anywhere from the second slot of
	// the epoch to the first slot of the next-but-one epoch.
	firstSlot := chainTime.FirstSlotOfEpoch(summary.Epoch) + 1
	lastSlot := chainTime.FirstSlotOfEpoch(summary.Epoch + 2)
	if lastSlot > chainTime.CurrentSlot() {
		lastSlot = chainTime.CurrentSlot()
	}

	// Obtain the duties for the validators to know where they should be attesting.
	dutiesResponse, err := attesterDutiesProvider.AttesterDuties(blockchain.Ctx, &api.AttesterDutiesOpts{
		Epoch:   summary.Epoch,
		Indices: activeValidatorIndices,
	})
	if err != nil {
		return errors.Wrap(err, "failed to obtain attester duties")
	}
	duties := dutiesResponse.Data
	for slot := chainTime.FirstSlotOfEpoch(summary.Epoch); slot < chainTime.FirstSlotOfEpoch(summary.Epoch+1); slot++ {
		index := int(slot - chainTime.FirstSlotOfEpoch(summary.Epoch))
		summary.Slots[index].Attestations = &slotAttestations{}
	}

	// Need a cache of beacon block headers to reduce lookup times.
	headersCache := util.NewBeaconBlockHeaderCache(beaconBlockHeadersProvider)

	// Need a map of duties to easily find the attestations we care about.
	dutiesBySlot := make(map[phase0.Slot]map[phase0.CommitteeIndex][]*apiv1.AttesterDuty)
	dutiesByValidatorIndex := make(map[phase0.ValidatorIndex]*apiv1.AttesterDuty)
	for _, duty := range duties {
		index := int(duty.Slot - chainTime.FirstSlotOfEpoch(summary.Epoch))
		dutiesByValidatorIndex[duty.ValidatorIndex] = duty
		summary.Slots[index].Attestations.Expected++
		if _, exists := dutiesBySlot[duty.Slot]; !exists {
			dutiesBySlot[duty.Slot] = make(map[phase0.CommitteeIndex][]*apiv1.AttesterDuty)
		}
		if _, exists := dutiesBySlot[duty.Slot][duty.CommitteeIndex]; !exists {
			dutiesBySlot[duty.Slot][duty.CommitteeIndex] = make([]*apiv1.AttesterDuty, 0)
		}
		dutiesBySlot[duty.Slot][duty.CommitteeIndex] = append(dutiesBySlot[duty.Slot][duty.CommitteeIndex], duty)
	}

	summary.IncorrectHeadValidators = make([]*validatorFault, 0)
	summary.UntimelyHeadValidators = make([]*validatorFault, 0)
	summary.UntimelySourceValidators = make([]*validatorFault, 0)
	summary.IncorrectTargetValidators = make([]*validatorFault, 0)
	summary.UntimelyTargetValidators = make([]*validatorFault, 0)

	// Hunt through the blocks looking for attestations from the validators.
	votes := make(map[phase0.ValidatorIndex]struct{})
	for slot := firstSlot; slot <= lastSlot; slot++ {
		if err := processAttesterDutiesSlot(slot, dutiesBySlot, votes, headersCache, activeValidatorIndices, summary); err != nil {
			return err
		}
	}

	// Use dutiesMap and votes to work out which validators didn't participate.
	summary.NonParticipatingValidators = make([]*nonParticipatingValidator, 0)
	for _, index := range activeValidatorIndices {
		if _, exists := votes[index]; !exists {
			// Didn't vote.
			duty := dutiesByValidatorIndex[index]
			summary.NonParticipatingValidators = append(summary.NonParticipatingValidators, &nonParticipatingValidator{
				Validator: index,
				Slot:      duty.Slot,
				Committee: duty.CommitteeIndex,
			})
		}
	}

	// Sort the non-participating validators list.
	sort.Slice(summary.NonParticipatingValidators, func(i int, j int) bool {
		if summary.NonParticipatingValidators[i].Slot != summary.NonParticipatingValidators[j].Slot {
			return summary.NonParticipatingValidators[i].Slot < summary.NonParticipatingValidators[j].Slot
		}
		if summary.NonParticipatingValidators[i].Committee != summary.NonParticipatingValidators[j].Committee {
			return summary.NonParticipatingValidators[i].Committee < summary.NonParticipatingValidators[j].Committee
		}
		return summary.NonParticipatingValidators[i].Validator < summary.NonParticipatingValidators[j].Validator
	})

	summary.ActiveValidators = len(activeValidators)
	summary.ParticipatingValidators = len(votes)

	return nil
}

func processAttesterDutiesSlot(
	slot phase0.Slot,
	dutiesBySlot map[phase0.Slot]map[phase0.CommitteeIndex][]*apiv1.AttesterDuty,
	votes map[phase0.ValidatorIndex]struct{},
	headersCache *util.BeaconBlockHeaderCache,
	activeValidatorIndices []phase0.ValidatorIndex,
	summary *validatorSummary,
) error {
	blockResponse, err := blocksProvider.SignedBeaconBlock(blockchain.Ctx, &api.SignedBeaconBlockOpts{
		Block: fmt.Sprintf("%d", slot),
	})
	if err != nil {
		var apiErr *api.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil
		}

		return errors.Wrap(err, "failed to obtain beacon block")
	}
	block := blockResponse.Data
	attestations, err := block.Attestations()
	if err != nil {
		return err
	}
	for _, attestation := range attestations {
		if _, exists := dutiesBySlot[attestation.Data.Slot]; !exists {
			// We do not have any attestations for this slot.
			continue
		}
		if _, exists := dutiesBySlot[attestation.Data.Slot][attestation.Data.Index]; !exists {
			// We do not have any attestations for this committee.
			continue
		}
		for _, duty := range dutiesBySlot[attestation.Data.Slot][attestation.Data.Index] {
			if attestation.AggregationBits.BitAt(duty.ValidatorCommitteeIndex) {
				// Found it.
				if _, exists := votes[duty.ValidatorIndex]; exists {
					// Duplicate; ignore.
					continue
				}
				votes[duty.ValidatorIndex] = struct{}{}

				// Update the metrics for the attestation.
				index := int(attestation.Data.Slot - chainTime.FirstSlotOfEpoch(summary.Epoch))
				summary.Slots[index].Attestations.Included++
				inclusionDelay := slot - duty.Slot

				fault := &validatorFault{
					Validator:         duty.ValidatorIndex,
					AttestationData:   attestation.Data,
					InclusionDistance: int(inclusionDelay),
				}

				headCorrect, err := AttestationHeadCorrect(blockchain.Ctx, headersCache, attestation)
				if err != nil {
					return errors.Wrap(err, "failed to calculate if attestation had correct head vote")
				}
				if headCorrect {
					summary.Slots[index].Attestations.CorrectHead++
					if inclusionDelay == 1 {
						summary.Slots[index].Attestations.TimelyHead++
					} else {
						summary.UntimelyHeadValidators = append(summary.UntimelyHeadValidators, fault)
					}
				} else {
					summary.IncorrectHeadValidators = append(summary.IncorrectHeadValidators, fault)
					if inclusionDelay > 1 {
						summary.UntimelyHeadValidators = append(summary.UntimelyHeadValidators, fault)
					}
				}

				if inclusionDelay <= 5 {
					summary.Slots[index].Attestations.TimelySource++
				} else {
					summary.UntimelySourceValidators = append(summary.UntimelySourceValidators, fault)
				}

				targetCorrect, err := AttestationTargetCorrect(blockchain.Ctx, headersCache, attestation)
				if err != nil {
					return errors.Wrap(err, "failed to calculate if attestation had correct target vote")
				}
				if targetCorrect {
					summary.Slots[index].Attestations.CorrectTarget++
					if inclusionDelay <= 32 {
						summary.Slots[index].Attestations.TimelyTarget++
					} else {
						summary.UntimelyTargetValidators = append(summary.UntimelyTargetValidators, fault)
					}
				} else {
					summary.IncorrectTargetValidators = append(summary.IncorrectTargetValidators, fault)
					if inclusionDelay > 32 {
						summary.UntimelyTargetValidators = append(summary.UntimelyTargetValidators, fault)
					}
				}
			}
		}

		if len(votes) == len(activeValidatorIndices) {
			// Found them all.
			break
		}
	}

	return nil
}

// AttestationHeadCorrect returns true if the given attestation had the correct head.
func AttestationHeadCorrect(ctx context.Context,
	headersCache *util.BeaconBlockHeaderCache,
	attestation *phase0.Attestation,
) (
	bool,
	error,
) {
	slot := attestation.Data.Slot
	for {
		header, err := headersCache.Fetch(ctx, slot)
		if err != nil {
			return false, err
		}
		if header == nil {
			// No block.
			slot--
			continue
		}
		if !header.Canonical {
			// Not canonical.
			slot--
			continue
		}
		return bytes.Equal(header.Root[:], attestation.Data.BeaconBlockRoot[:]), nil
	}
}

// AttestationTargetCorrect returns true if the given attestation had the correct target.
func AttestationTargetCorrect(ctx context.Context,
	headersCache *util.BeaconBlockHeaderCache,
	attestation *phase0.Attestation,
) (
	bool,
	error,
) {
	// Start with first slot of the target epoch.
	slot := chainTime.FirstSlotOfEpoch(attestation.Data.Target.Epoch)
	for {
		header, err := headersCache.Fetch(ctx, slot)
		if err != nil {
			return false, err
		}
		if header == nil {
			// No block.
			slot--
			continue
		}
		if !header.Canonical {
			// Not canonical.
			slot--
			continue
		}
		return bytes.Equal(header.Root[:], attestation.Data.Target.Root[:]), nil
	}
}

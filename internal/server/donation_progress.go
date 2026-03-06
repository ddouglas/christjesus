package server

import (
	"context"

	"christjesus/pkg/types"
)

func (s *Service) applyFinalizedRaisedAmount(ctx context.Context, need *types.Need) error {
	if need == nil {
		return nil
	}

	amountRaisedCents, err := s.donationIntentRepo.FinalizedAmountByNeedID(ctx, need.ID)
	if err != nil {
		return err
	}

	need.AmountRaisedCents = amountRaisedCents
	return nil
}

func (s *Service) applyFinalizedRaisedAmounts(ctx context.Context, needs []*types.Need) error {
	if len(needs) == 0 {
		return nil
	}

	needIDs := make([]string, 0, len(needs))
	for _, need := range needs {
		if need == nil || need.ID == "" {
			continue
		}
		needIDs = append(needIDs, need.ID)
	}
	if len(needIDs) == 0 {
		return nil
	}

	amountsByNeedID, err := s.donationIntentRepo.FinalizedAmountsByNeedIDs(ctx, needIDs)
	if err != nil {
		return err
	}

	for _, need := range needs {
		if need == nil || need.ID == "" {
			continue
		}
		need.AmountRaisedCents = amountsByNeedID[need.ID]
	}

	return nil
}

func fundingPercentFromCents(amountRaisedCents, amountNeededCents int) int {
	if amountNeededCents <= 0 {
		return 0
	}

	fundingPercent := (amountRaisedCents * 100) / amountNeededCents
	if fundingPercent < 0 {
		fundingPercent = 0
	}
	if fundingPercent > 100 {
		fundingPercent = 100
	}
	return fundingPercent
}

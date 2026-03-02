package seed

import (
	"christjesus/internal/store"
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var fakeNeedDescriptions = []string{
	"Need support to stabilize housing while starting a new job.",
	"Seeking help with medical costs and transportation to treatment.",
	"Requesting assistance for groceries and essential nutrition items.",
	"Catching up on utilities after a temporary income interruption.",
	"Need childcare support during transition back to full-time work.",
	"Trying to replace critical work tools to maintain employment.",
	"Need temporary support for rent and safety-related expenses.",
	"Requesting help with educational supplies and certification fees.",
	"Need help covering emergency bills after a family crisis.",
	"Seeking transportation support to reach work and appointments.",
}

type weightedNeedStatus struct {
	Status types.NeedStatus
	Weight int
}

var weightedStatuses = []weightedNeedStatus{
	{Status: types.NeedStatusDraft, Weight: 35},
	{Status: types.NeedStatusSubmitted, Weight: 20},
	{Status: types.NeedStatusUnderReview, Weight: 15},
	{Status: types.NeedStatusApproved, Weight: 10},
	{Status: types.NeedStatusActive, Weight: 12},
	{Status: types.NeedStatusFunded, Weight: 8},
}

func SeedFakeNeeds(
	ctx context.Context,
	pool *pgxpool.Pool,
	needsRepo *store.NeedRepository,
	categoryRepo *store.CategoryRepository,
	assignmentRepo *store.AssignmentRepository,
	storyRepo *store.StoryRepository,
	count int,
	reset bool,
) error {
	if count <= 0 {
		fmt.Println("Skipping fake needs seed because count <= 0")
		return nil
	}

	if reset {
		result, err := pool.Exec(ctx, `DELETE FROM christjesus.needs WHERE short_description LIKE '[seed] %'`)
		if err != nil {
			return fmt.Errorf("failed to reset seeded fake needs: %w", err)
		}
		fmt.Printf("Reset seeded fake needs: %d deleted\n", result.RowsAffected())
	}

	categories, err := categoryRepo.Categories(ctx)
	if err != nil {
		return fmt.Errorf("failed to load categories for fake needs: %w", err)
	}

	if len(categories) == 0 {
		return fmt.Errorf("no categories found; run category seed first")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	fakeNeedUserIDs := seedFakeNeedUserIDs()
	if len(fakeNeedUserIDs) == 0 {
		return fmt.Errorf("no fake need users available; seed fake users first")
	}

	created := 0
	for i := 0; i < count; i++ {
		status := pickWeightedStatus(rng)
		amountNeeded := (rng.Intn(2400) + 100) * 100

		amountRaised := 0
		switch status {
		case types.NeedStatusFunded:
			amountRaised = amountNeeded
		case types.NeedStatusActive, types.NeedStatusApproved:
			amountRaised = rng.Intn(max(amountNeeded-100, 100))
		case types.NeedStatusUnderReview, types.NeedStatusSubmitted:
			amountRaised = rng.Intn(max(amountNeeded/3, 100))
		}

		shortDescription := fmt.Sprintf("[seed] %s", fakeNeedDescriptions[rng.Intn(len(fakeNeedDescriptions))])
		currentStep := stepForStatus(status, rng)

		need := &types.Need{
			UserID:            fakeNeedUserIDs[rng.Intn(len(fakeNeedUserIDs))],
			AmountNeededCents: amountNeeded,
			AmountRaisedCents: amountRaised,
			ShortDescription:  utils.StringPtr(shortDescription),
			Status:            status,
			CurrentStep:       currentStep,
			IsFeatured:        status == types.NeedStatusActive && rng.Intn(100) < 15,
		}

		now := time.Now()
		if status != types.NeedStatusDraft {
			need.SubmittedAt = utils.TimePtr(now.Add(-time.Duration(rng.Intn(14*24)) * time.Hour))
		}
		if status == types.NeedStatusApproved || status == types.NeedStatusActive || status == types.NeedStatusFunded {
			verifiedBy := fakeNeedUserIDs[rng.Intn(len(fakeNeedUserIDs))]
			need.VerifiedBy = utils.StringPtr(verifiedBy)
			need.VerifiedAt = utils.TimePtr(now.Add(-time.Duration(rng.Intn(10*24)) * time.Hour))
		}
		if status == types.NeedStatusActive || status == types.NeedStatusFunded {
			need.PublishedAt = utils.TimePtr(now.Add(-time.Duration(rng.Intn(7*24)) * time.Hour))
		}

		if err := needsRepo.CreateNeed(ctx, need); err != nil {
			return fmt.Errorf("failed to create fake need %d: %w", i+1, err)
		}

		primary := categories[rng.Intn(len(categories))]
		assignments := []*types.NeedCategoryAssignment{{
			NeedID:     need.ID,
			CategoryID: primary.ID,
			IsPrimary:  true,
		}}

		secondaryCount := rng.Intn(3)
		used := map[string]bool{primary.ID: true}
		for j := 0; j < secondaryCount; j++ {
			candidate := categories[rng.Intn(len(categories))]
			if used[candidate.ID] {
				continue
			}
			used[candidate.ID] = true
			assignments = append(assignments, &types.NeedCategoryAssignment{
				NeedID:     need.ID,
				CategoryID: candidate.ID,
				IsPrimary:  false,
			})
		}

		if err := assignmentRepo.CreateAssignments(ctx, assignments); err != nil {
			return fmt.Errorf("failed to create category assignments for fake need %s: %w", need.ID, err)
		}

		if rng.Intn(100) < 85 {
			storyCurrent := "Current situation has created urgent financial pressure for this household."
			storyNeed := "This support would cover immediate essentials and prevent further instability."
			storyOutcome := "With timely support, this need can move toward stability within the next month."

			story := &types.NeedStory{
				NeedID:  need.ID,
				Current: utils.StringPtr(storyCurrent),
				Need:    utils.StringPtr(storyNeed),
				Outcome: utils.StringPtr(storyOutcome),
			}

			if err := storyRepo.CreateStory(ctx, story); err != nil {
				return fmt.Errorf("failed to create story for fake need %s: %w", need.ID, err)
			}
		}

		created++
	}

	fmt.Printf("Fake needs seeded: %d created\n", created)
	return nil
}

func pickWeightedStatus(rng *rand.Rand) types.NeedStatus {
	total := 0
	for _, item := range weightedStatuses {
		total += item.Weight
	}

	if total == 0 {
		return types.NeedStatusDraft
	}

	roll := rng.Intn(total)
	running := 0
	for _, item := range weightedStatuses {
		running += item.Weight
		if roll < running {
			return item.Status
		}
	}

	return types.NeedStatusDraft
}

func stepForStatus(status types.NeedStatus, rng *rand.Rand) types.NeedStep {
	switch status {
	case types.NeedStatusDraft:
		steps := []types.NeedStep{types.NeedStepWelcome, types.NeedStepLocation, types.NeedStepCategories, types.NeedStepStory, types.NeedStepDocuments, types.NeedStepReview}
		return steps[rng.Intn(len(steps))]
	case types.NeedStatusSubmitted, types.NeedStatusUnderReview:
		return types.NeedStepReview
	case types.NeedStatusApproved, types.NeedStatusActive, types.NeedStatusFunded:
		return types.NeedStepComplete
	default:
		return types.NeedStepReview
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

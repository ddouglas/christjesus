package seed

import (
	"christjesus/internal/store"
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
)

// SeedCategories syncs the database with the category definitions below.
// This file is the source of truth for categories:
// - Inserts new categories that don't exist
// - Updates existing categories that have changed
// - Deletes categories from DB that aren't in this list
//
// To generate new IDs: `go run ./cmd/christjesus nanoid`
// To add a category: Add it to the list with a new ID and run `just seed`
// To remove a category: Remove it from the list and run `just seed` (auto-deleted from DB)
// To update a category: Edit the fields and run `just seed`
func SeedCategories(ctx context.Context, repo *store.CategoryRepository) error {
	// Define seed data with fixed IDs
	// compile-time safe - if NeedCategory type changes, this won't compile
	categories := []types.NeedCategory{
		{
			ID:           "ehAIZ65SBy8ewyOWiJVpRdP9W78STAse",
			Name:         "Housing & Shelter",
			Slug:         "housing-shelter",
			Description:  utils.StringPtr("Assistance with rent, mortgage, temporary housing, or shelter needs"),
			Icon:         utils.StringPtr("home"),
			DisplayOrder: 1,
			IsActive:     true,
		},
		{
			ID:           "MkeMQP08IH9k5rXHspDUer2xQWOLjHza",
			Name:         "Food & Nutrition",
			Slug:         "food-nutrition",
			Description:  utils.StringPtr("Help with groceries, meal assistance, or nutrition support"),
			Icon:         utils.StringPtr("utensils"),
			DisplayOrder: 2,
			IsActive:     true,
		},
		{
			ID:           "8kzJOd6irR67jH2MPq8LxoqIK7tJ3CV6",
			Name:         "Medical & Healthcare",
			Slug:         "medical-healthcare",
			Description:  utils.StringPtr("Medical bills, prescriptions, treatments, or health-related expenses"),
			Icon:         utils.StringPtr("heart-pulse"),
			DisplayOrder: 3,
			IsActive:     true,
		},
		{
			ID:           "2gT3IW1x9HoTYADSiT7TWDtFby8f4ccx",
			Name:         "Utilities & Bills",
			Slug:         "utilities-bills",
			Description:  utils.StringPtr("Electricity, water, gas, phone, internet, or other essential services"),
			Icon:         utils.StringPtr("lightbulb"),
			DisplayOrder: 4,
			IsActive:     true,
		},
		{
			ID:           "0Yis9XuFbdESHRF8yNRt4vzHfBEUZzVt",
			Name:         "Transportation",
			Slug:         "transportation",
			Description:  utils.StringPtr("Vehicle repairs, gas, public transportation, or mobility assistance"),
			Icon:         utils.StringPtr("car"),
			DisplayOrder: 5,
			IsActive:     true,
		},
		{
			ID:           "CWRY9NcTDtKgW5kY5hj6vgejsRQ5SBsv",
			Name:         "Employment & Income",
			Slug:         "employment-income",
			Description:  utils.StringPtr("Job training, work tools, professional clothing, or income gap support"),
			Icon:         utils.StringPtr("briefcase"),
			DisplayOrder: 6,
			IsActive:     true,
		},
		{
			ID:           "SbikmS7HyVZOusy0MFcHVJpBVCqd6CQd",
			Name:         "Education & Training",
			Slug:         "education-training",
			Description:  utils.StringPtr("School supplies, tuition, books, or vocational training costs"),
			Icon:         utils.StringPtr("book"),
			DisplayOrder: 7,
			IsActive:     true,
		},
		{
			ID:           "Lkk49SMHJ1x91O2Nn16zPFkw2ZUfFHov",
			Name:         "Family & Childcare",
			Slug:         "family-childcare",
			Description:  utils.StringPtr("Childcare, baby supplies, family support, or parenting resources"),
			Icon:         utils.StringPtr("users"),
			DisplayOrder: 8,
			IsActive:     true,
		},
		{
			ID:           "U4gsVC2u4b7YfixUIJKvWLib2YvYwcP4",
			Name:         "Legal & Documentation",
			Slug:         "legal-documentation",
			Description:  utils.StringPtr("Legal fees, ID documentation, immigration support, or court costs"),
			Icon:         utils.StringPtr("file-text"),
			DisplayOrder: 9,
			IsActive:     true,
		},
		{
			ID:           "3O25B8RXOmCmhFiBqVS99wvOMhRmOWsH",
			Name:         "Emergency & Crisis",
			Slug:         "emergency-crisis",
			Description:  utils.StringPtr("Urgent, immediate needs requiring quick assistance"),
			Icon:         utils.StringPtr("alert-circle"),
			DisplayOrder: 10,
			IsActive:     true,
		},
	}

	fmt.Println("Starting category sync...")
	fmt.Printf("  Seed file contains %d categories\n", len(categories))

	// Build a map of IDs from seed data for quick lookup
	seedIDs := make(map[string]bool)
	for _, cat := range categories {
		seedIDs[cat.ID] = true
	}

	// Get ALL categories from database (including inactive)
	existing, err := repo.AllCategoriesUnfiltered(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch existing categories: %w", err)
	}
	fmt.Printf("  Database contains %d categories\n", len(existing))

	// Delete categories that exist in DB but not in seed file
	deletedCount := 0
	for _, existingCat := range existing {
		if !seedIDs[existingCat.ID] {
			fmt.Printf("  Deleting category: %s (id: %s)\n", existingCat.Name, existingCat.ID)
			if err := repo.DeleteCategory(ctx, existingCat.ID); err != nil {
				return fmt.Errorf("failed to delete category %s: %w", existingCat.ID, err)
			}
			deletedCount++
		}
	}

	// Upsert each category from seed file (insert or update)
	upsertedCount := 0
	for _, cat := range categories {
		fmt.Printf("  Upserting category: %s (slug: %s)\n", cat.Name, cat.Slug)
		if err := repo.UpsertCategory(ctx, &cat); err != nil {
			return fmt.Errorf("failed to upsert category %s: %w", cat.Slug, err)
		}
		upsertedCount++
	}

	fmt.Printf("\nSync complete: %d upserted, %d deleted\n", upsertedCount, deletedCount)
	return nil
}

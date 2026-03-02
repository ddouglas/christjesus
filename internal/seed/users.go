package seed

import (
	"christjesus/internal/store"
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"errors"
	"fmt"
)

type fakeUserSeed struct {
	ID         string
	Email      string
	GivenName  string
	FamilyName string
	UserType   types.UserType
}

var fakeUsers = []fakeUserSeed{
	{ID: "11111111-1111-1111-1111-111111111111", Email: "ava.williams+seed1@example.com", GivenName: "Ava", FamilyName: "Williams", UserType: types.UserTypeNeed},
	{ID: "22222222-2222-2222-2222-222222222222", Email: "liam.johnson+seed2@example.com", GivenName: "Liam", FamilyName: "Johnson", UserType: types.UserTypeNeed},
	{ID: "33333333-3333-3333-3333-333333333333", Email: "noah.brown+seed3@example.com", GivenName: "Noah", FamilyName: "Brown", UserType: types.UserTypeNeed},
	{ID: "44444444-4444-4444-4444-444444444444", Email: "mia.davis+seed4@example.com", GivenName: "Mia", FamilyName: "Davis", UserType: types.UserTypeNeed},
	{ID: "55555555-5555-5555-5555-555555555555", Email: "elijah.garcia+seed5@example.com", GivenName: "Elijah", FamilyName: "Garcia", UserType: types.UserTypeNeed},
	{ID: "66666666-6666-6666-6666-666666666666", Email: "olivia.miller+seed6@example.com", GivenName: "Olivia", FamilyName: "Miller", UserType: types.UserTypeNeed},
	{ID: "77777777-7777-7777-7777-777777777777", Email: "ethan.moore+seed7@example.com", GivenName: "Ethan", FamilyName: "Moore", UserType: types.UserTypeNeed},
	{ID: "88888888-8888-8888-8888-888888888888", Email: "sophia.taylor+seed8@example.com", GivenName: "Sophia", FamilyName: "Taylor", UserType: types.UserTypeNeed},
}

func seedFakeNeedUserIDs() []string {
	ids := make([]string, 0, len(fakeUsers))
	for _, user := range fakeUsers {
		if user.UserType == types.UserTypeNeed {
			ids = append(ids, user.ID)
		}
	}
	return ids
}

func SeedFakeUsers(ctx context.Context, userRepo *store.UserRepository) error {
	seeded := 0
	for _, fakeUser := range fakeUsers {
		existing, err := userRepo.User(ctx, fakeUser.ID)
		if err != nil {
			if !errors.Is(err, types.ErrUserNotFound) {
				return fmt.Errorf("failed to fetch fake user %s: %w", fakeUser.ID, err)
			}

			userType := string(fakeUser.UserType)
			newUser := &types.User{
				ID:         fakeUser.ID,
				UserType:   &userType,
				Email:      utils.StringPtr(fakeUser.Email),
				GivenName:  utils.StringPtr(fakeUser.GivenName),
				FamilyName: utils.StringPtr(fakeUser.FamilyName),
			}

			if err := userRepo.Create(ctx, newUser); err != nil {
				return fmt.Errorf("failed to create fake user %s: %w", fakeUser.ID, err)
			}
			seeded++
			continue
		}

		userType := string(fakeUser.UserType)
		existing.UserType = &userType
		existing.Email = utils.StringPtr(fakeUser.Email)
		existing.GivenName = utils.StringPtr(fakeUser.GivenName)
		existing.FamilyName = utils.StringPtr(fakeUser.FamilyName)

		if err := userRepo.Update(ctx, fakeUser.ID, existing); err != nil {
			return fmt.Errorf("failed to update fake user %s: %w", fakeUser.ID, err)
		}
		seeded++
	}

	fmt.Printf("Fake users seeded: %d upserted\n", seeded)
	return nil
}

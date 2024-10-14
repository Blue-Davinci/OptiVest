package data

import (
	"context"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
)

type AwardManagerModel struct {
	DB *database.Queries
}

const (
	DefaultAwManDBContextTimeout = 5 * time.Second
)

type Award struct {
	ID          int32     `json:"id"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateNewUserAward() is a method that creates a new user award
// We accept a user ID and an award ID
// We return a created at and an error if there is one
func (m AwardManagerModel) CreateNewUserAward(userID int64, awardID int32) (time.Time, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultAwManDBContextTimeout)
	defer cancel()
	// create a new user award
	createdAt, err := m.DB.CreateNewUserAward(ctx, database.CreateNewUserAwardParams{
		UserID:  userID,
		AwardID: awardID,
	})
	if err != nil {
		return time.Time{}, err
	}
	return createdAt, nil
}

// GetAllAwards() is a method that returns all the awards
// We return a *slice of awards and an error if there is one
func (m AwardManagerModel) GetAllAwards() ([]*Award, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultAwManDBContextTimeout)
	defer cancel()
	// get all the awards
	awardRows, err := m.DB.GetAllAwards(ctx)
	if err != nil {
		return nil, err
	}
	// make a slice of awards
	awards := []*Award{}
	// iterate through the rows and append the awards
	for _, awardRow := range awardRows {
		// populate award
		award := populateAward(awardRow)
		if award != nil {
			awards = append(awards, award)
		}
	}
	return awards, nil
}

// GetAllAwardsForUserByID() is a method that returns all the awards for a user by ID
// We accept a user ID
// We return a *slice of awards and an error if there is one
func (m AwardManagerModel) GetAllAwardsForUserByID(userID int64) ([]*Award, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultAwManDBContextTimeout)
	defer cancel()
	// get all the awards for a user by ID
	awardRows, err := m.DB.GetAllAwardsForUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// make a slice of awards
	awards := []*Award{}
	// iterate through the rows and append the awards
	for _, awardRow := range awardRows {
		// populate award
		award := populateAward(awardRow)
		if award != nil {
			awards = append(awards, award)
		}
	}
	return awards, nil
}

// populateAward() is a method that populates an award
func populateAward(awardRow interface{}) *Award {
	// populate award
	switch award := awardRow.(type) {
	case database.Award:
		return &Award{
			ID:          award.ID,
			Code:        award.Code,
			Description: award.Description,
			CreatedAt:   award.CreatedAt,
			UpdatedAt:   award.UpdatedAt,
		}
	default:
		return nil
	}
}

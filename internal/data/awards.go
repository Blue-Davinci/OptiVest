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
	ID            int32     `json:"id"`
	Code          string    `json:"code"`
	Description   string    `json:"description"`
	AwardImageUrl string    `json:"award_image_url"`
	Points        int32     `json:"points"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateNewUserAward creates a new user award row. ctx flows from the
// caller (typically a goroutine driven by the awards background scheduler
// or a request handler).
func (m AwardManagerModel) CreateNewUserAward(ctx context.Context, userID int64, awardID int32) (time.Time, error) {
	ctx, cancel := contextGenerator(ctx, DefaultAwManDBContextTimeout)
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

// GetAwardByAwardID returns an award by ID. ctx flows from the caller.
func (m AwardManagerModel) GetAwardByAwardID(ctx context.Context, awardID int32) (*Award, error) {
	ctx, cancel := contextGenerator(ctx, DefaultAwManDBContextTimeout)
	defer cancel()
	// get an award by ID
	awardRow, err := m.DB.GetAwardByAwardID(ctx, awardID)
	if err != nil {
		return nil, err
	}
	// populate award
	award := populateAward(awardRow)
	return award, nil
}

// GetAllAwards returns all awards. ctx flows from the caller.
func (m AwardManagerModel) GetAllAwards(ctx context.Context) ([]*Award, error) {
	ctx, cancel := contextGenerator(ctx, DefaultAwManDBContextTimeout)
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

// GetAllAwardsForUserByID returns all the awards for a user by ID. ctx
// flows from the caller.
func (m AwardManagerModel) GetAllAwardsForUserByID(ctx context.Context, userID int64) ([]*Award, error) {
	ctx, cancel := contextGenerator(ctx, DefaultAwManDBContextTimeout)
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
			ID:            award.ID,
			Code:          award.Code,
			Description:   award.Description,
			AwardImageUrl: award.AwardImageUrl.String,
			Points:        award.Points,
			CreatedAt:     award.CreatedAt,
			UpdatedAt:     award.UpdatedAt,
		}
	default:
		return nil
	}
}

package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

type CommentManagerModel struct {
	DB *database.Queries
}

const (
	DefaultCommManDBContextTimeout = 5 * time.Second
)

var (
	ErrInvalidAssociatedType = errors.New("invalid associated type")
)

const (
	CommentAssociatedTypeFeed  = database.CommentAssociatedTypeFeed
	CommentAssociatedTypeGroup = database.CommentAssociatedTypeGroup
	CommentAssociatedTypeOther = database.CommentAssociatedTypeOther
)

type Comment struct {
	ID             int64                          `json:"id"`
	Content        string                         `json:"content"`
	UserID         int64                          `json:"user_id"`
	ParentID       int64                          `json:"parent_id"`
	AssociatedType database.CommentAssociatedType `json:"associated_type"`
	AssociatedID   int64                          `json:"associated_id"`
	CreatedAt      time.Time                      `json:"created_at"`
	UpdatedAt      time.Time                      `json:"updated_at"`
	Version        int32                          `json:"version"`
}

// MapCommentTypeToConst maps the comment type to the database constant
func (m CommentManagerModel) MapCommentTypeToConst(commentType string) (database.CommentAssociatedType, error) {
	switch commentType {
	case "feed":
		return CommentAssociatedTypeFeed, nil
	case "group":
		return CommentAssociatedTypeGroup, nil
	case "other":
		return CommentAssociatedTypeOther, nil
	default:
		return "", ErrInvalidAssociatedType
	}
}

func ValidateComment(v *validator.Validator, comment *Comment) {
	ValidateName(v, comment.Content, "content")
	ValidateName(v, string(comment.AssociatedType), "associated_type")
}

// CreateNewComment creates a new comment in the database
// We take in the user ID and the comment struct and return an error if there is one
func (m CommentManagerModel) CreateNewComment(userID int64, comment *Comment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// insert
	createdFeed, err := m.DB.CreateNewComment(ctx, database.CreateNewCommentParams{
		Content:        comment.Content,
		UserID:         userID,
		ParentID:       sql.NullInt64{Int64: comment.ParentID, Valid: comment.ParentID != 0},
		AssociatedType: comment.AssociatedType,
		AssociatedID:   comment.AssociatedID,
	})
	if err != nil {
		return err
	}
	// fill in the comment
	comment.ID = createdFeed.ID
	comment.UserID = userID
	comment.CreatedAt = createdFeed.CreatedAt.Time
	comment.UpdatedAt = createdFeed.UpdatedAt.Time
	// return
	return nil
}

// UpdateComment updates a comment in the database
// We take in the user ID and the comment struct which includes the comment and version
// We return an error if there is one
func (m CommentManagerModel) UpdateComment(userID int64, comment *Comment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// update
	updatedFeed, err := m.DB.UpdateComment(ctx, database.UpdateCommentParams{
		ID:      comment.ID,
		UserID:  userID,
		Version: sql.NullInt32{Int32: comment.Version, Valid: true},
		Content: comment.Content,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// fill in the comment
	comment.UpdatedAt = updatedFeed.UpdatedAt.Time
	comment.Version = updatedFeed.Version.Int32
	// return
	return nil
}

// DeleteComment deletes a comment in the database
// We take in the user ID and the comment ID and return an error if there is one
func (m CommentManagerModel) DeleteComment(userID, commentID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// delete
	_, err := m.DB.DeleteComment(ctx, database.DeleteCommentParams{
		ID:     commentID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrGeneralRecordNotFound
		default:
			return err
		}
	}
	// return
	return nil
}

// GetCommentById gets a comment by its ID
// We take in the comment ID and return the comment and an error if there is one
func (m CommentManagerModel) GetCommentById(userID, commentID int64) (*Comment, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// get the comment
	commentRow, err := m.DB.GetCommentById(ctx, database.GetCommentByIdParams{
		ID:     commentID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// fill in the comment
	comment := populateComment(commentRow)
	// return
	return comment, nil
}

func populateComment(commentRow interface{}) *Comment {
	switch comment := commentRow.(type) {
	case database.Comment:
		return &Comment{
			ID:             comment.ID,
			Content:        comment.Content,
			UserID:         comment.UserID,
			ParentID:       comment.ParentID.Int64,
			AssociatedType: comment.AssociatedType,
			AssociatedID:   comment.AssociatedID,
			CreatedAt:      comment.CreatedAt.Time,
			UpdatedAt:      comment.UpdatedAt.Time,
			Version:        comment.Version.Int32,
		}
	default:
		return nil
	}

}

package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	ErrDuplicateReaction     = errors.New("duplicate reaction")
)

const (
	CommentAssociatedTypeFeed  = database.CommentAssociatedTypeFeed
	CommentAssociatedTypeGroup = database.CommentAssociatedTypeGroup
	CommentAssociatedTypeOther = database.CommentAssociatedTypeOther
)

type OrganizedComment struct {
	Parent  *EnrichedComment   `json:"parent"`
	Replies []*EnrichedComment `json:"replies"`
}

// EnrichedComment represents a comment with additional user & reaction information
type EnrichedComment struct {
	UserName      string   `json:"user_name"`
	UserAvatar    string   `json:"user_avatar"`
	UserRole      string   `json:"user_role,omitempty"` // omit for non group comments
	HasUserLiked  bool     `json:"has_user_liked"`      // true if the user has liked the comment
	ReactionCount int64    `json:"reaction_count"`
	Comment       *Comment `json:"comment"`
}

// Comment represents a comment in the database
type Comment struct {
	ID             int64                          `json:"id"`
	Content        string                         `json:"content"`
	UserID         int64                          `json:"user_id"`
	ParentID       int64                          `json:"parent_id"`
	AssociatedType database.CommentAssociatedType `json:"associated_type"`
	AssociatedID   int64                          `json:"associated_id"`
	CreatedAt      time.Time                      `json:"created_at"`
	UpdatedAt      time.Time                      `json:"updated_at"`
	Version        int32                          `json:"version,omitempty"` // only used for updates
}

type CommentReaction struct {
	ID        int64     `json:"id"`
	CommentID int64     `json:"comment_id"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

// ValidateComment validates the comment struct
func ValidateComment(v *validator.Validator, comment *Comment) {
	ValidateName(v, comment.Content, "content")
	ValidateName(v, string(comment.AssociatedType), "associated_type")
}

// ValidateCommentReaction validates the comment reaction struct
func ValidateCommentReaction(v *validator.Validator, reaction *CommentReaction) {
	ValidateURLID(v, reaction.CommentID, "comment_id")
}

// GetCommentsWithReactionsByAssociatedId gets all comments with reactions and user information
// We take in the user ID, the associated type, and the associated ID and return an enriched comment slice and an error if there is one
func (m CommentManagerModel) GetCommentsWithReactionsByAssociatedId(userID int64, associatedType database.CommentAssociatedType, associatedID int64) ([]*EnrichedComment, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// get the comments
	comments, err := m.DB.GetCommentsWithReactionsByAssociatedId(ctx, database.GetCommentsWithReactionsByAssociatedIdParams{
		UserID:         userID,
		AssociatedType: associatedType,
		AssociatedID:   associatedID,
	})
	if err != nil {
		return nil, err
	}
	// create the enriched comments
	enrichedComments := make([]*EnrichedComment, len(comments))
	for i, comment := range comments {
		enrichedComments[i] = &EnrichedComment{
			UserName:      fmt.Sprintf("%s %s", comment.FirstName, comment.LastName),
			UserAvatar:    comment.ProfileAvatarUrl,
			UserRole:      string(comment.UserRole),
			HasUserLiked:  comment.LikedByRequestingUser,
			ReactionCount: comment.LikesCount,
			Comment:       populateComment(comment),
		}
	}
	// return
	return enrichedComments, nil
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
		switch {
		case err.Error() == `pq: insert or update on table "comments" violates foreign key constraint "comments_parent_id_fkey"`:
			return ErrGeneralRecordNotFound
		default:
			return err
		}
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

// CreateNewReaction creates a new reaction for a specific comment ID
// We take in the user ID and a commentreaction struct and return an error if there is one
func (m CommentManagerModel) CreateNewReaction(userID int64, reaction *CommentReaction) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// insert
	createdReaction, err := m.DB.CreateNewReaction(ctx, database.CreateNewReactionParams{
		CommentID: reaction.CommentID,
		UserID:    userID,
	})
	if err != nil {
		switch {
		// "pq: duplicate key value violates unique constraint \"comment_reactions_comment_id_user_id_key\""
		case err.Error() == `pq: duplicate key value violates unique constraint "comment_reactions_comment_id_user_id_key"`:
			return ErrDuplicateReaction
		default:
			return err
		}
	}
	// fill in the reaction
	reaction.ID = createdReaction.ID
	reaction.UserID = userID
	reaction.CreatedAt = createdReaction.CreatedAt.Time
	reaction.UpdatedAt = createdReaction.UpdatedAt.Time
	// return
	return nil
}

// DeleteReaction deletes a reaction for a specific comment ID
// We take the userID and the Comment ID and return an error if there is one
func (m CommentManagerModel) DeleteReaction(userID, commentID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultCommManDBContextTimeout)
	defer cancel()
	// delete
	_, err := m.DB.DeleteReaction(ctx, database.DeleteReactionParams{
		CommentID: commentID,
		UserID:    userID,
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

// populateComment() populates a comment struct from a database comment
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
	case database.GetCommentsWithReactionsByAssociatedIdRow:
		return &Comment{
			ID:             comment.CommentID,
			Content:        comment.Content,
			UserID:         comment.UserID,
			ParentID:       comment.ParentID.Int64,
			AssociatedType: comment.AssociatedType,
			AssociatedID:   comment.AssociatedID,
			CreatedAt:      comment.CreatedAt.Time,
			UpdatedAt:      comment.UpdatedAt.Time,
		}
	default:
		return nil
	}

}

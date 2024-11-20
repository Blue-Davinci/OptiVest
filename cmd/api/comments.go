package main

import (
	"errors"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

// getCommentsWithReactionsByAssociatedIdHandler() returns all comments for a given entity/post with reactions
// we take in the associated ID and type from the URL and return all comments with reactions
func (app *application) getCommentsWithReactionsByAssociatedIdHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AssociatedID   int    `json:"associated_id"`
		AssociatedType string `json:"associated_type"`
	}
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	input.AssociatedID = app.readInt(qs, "associated_id", 0, v)
	input.AssociatedType = app.readString(qs, "associated_type", "")
	// validate the associated type
	commentType, err := app.models.CommentManagerModel.MapCommentTypeToConst(input.AssociatedType)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// validate associated ID
	if data.ValidateURLID(v, int64(input.AssociatedID), "associated_id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the comments
	comments, err := app.models.CommentManagerModel.GetCommentsWithReactionsByAssociatedId(app.contextGetUser(r).ID, commentType, int64(input.AssociatedID))
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	organizedComments := app.groupAndNestComments(comments)
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"comments": organizedComments}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewCommentHandler() creates a new comment in the database
// we take in a comment directly from the body, pass in the user context, and return the comment
func (app *application) createNewCommentHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content        string `json:"content"`         // comment content
		ParentID       int64  `json:"parent_id"`       // parent comment ID if this is a reply
		AssociatedType string `json:"associated_type"` // feed, group, other
		AssociatedID   int64  `json:"associated_id"`   // entity/post we are replying to
	}
	// read the input from the body
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// map the comment type to the database constant
	commentType, err := app.models.CommentManagerModel.MapCommentTypeToConst(input.AssociatedType)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// create the comment
	comment := &data.Comment{
		Content:        input.Content,
		ParentID:       input.ParentID,
		AssociatedType: commentType,
		AssociatedID:   input.AssociatedID,
	}
	// validate the comment
	v := validator.New()
	if data.ValidateComment(v, comment); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create the comment
	err = app.models.CommentManagerModel.CreateNewComment(app.contextGetUser(r).ID, comment)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"comment": comment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateCommentHandler() updates a comment in the database
// we take in the comment ID and the new content, validate the content, and return the updated comment
// The expected body should only contain the new content and version. The ID is passed in the URL
func (app *application) updateCommentHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content string `json:"content"`
		Version int32  `json:"version"`
	}
	// read the input from the body
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get the comment ID from the URL
	commentID, err := app.readIDParam(r, "commentID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// get the comment from the database
	comment, err := app.models.CommentManagerModel.GetCommentById(app.contextGetUser(r).ID, commentID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// update the comment
	comment.Content = input.Content
	comment.Version = input.Version
	// validate the comment
	v := validator.New()
	if data.ValidateComment(v, comment); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the comment
	err = app.models.CommentManagerModel.UpdateComment(app.contextGetUser(r).ID, comment)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"comment": comment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewReactionHandler() creates a new reaction/like for a comment
// We simply just take in the commentID in the body and return the reaction
func (app *application) createNewReactionHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CommentID int64 `json:"comment_id"`
	}
	// read the input from the body
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make the coment reaction
	reaction := &data.CommentReaction{
		CommentID: input.CommentID,
	}
	// validate the reaction
	v := validator.New()
	if data.ValidateCommentReaction(v, reaction); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// check if the comment exists
	_, err = app.models.CommentManagerModel.GetCommentById(app.contextGetUser(r).ID, reaction.CommentID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// create the reaction
	err = app.models.CommentManagerModel.CreateNewReaction(app.contextGetUser(r).ID, reaction)
	if err != nil {
		switch {
		// handle duplicate reaction errors
		case errors.Is(err, data.ErrDuplicateReaction):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"reaction": reaction}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteCommentHandler() deletes a comment in the database
// we take in the comment ID from the URL and return a 200 status if successful
func (app *application) deleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	// get the comment ID from the URL
	commentID, err := app.readIDParam(r, "commentID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// validate url ID
	v := validator.New()
	if data.ValidateURLID(v, commentID, "comment_id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the comment
	err = app.models.CommentManagerModel.DeleteComment(app.contextGetUser(r).ID, commentID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "comment successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteReactionHandler() deletes a reaction/like for a comment
// we take in the comment ID from the URL and return a 200 status if successful
func (app *application) deleteReactionHandler(w http.ResponseWriter, r *http.Request) {
	// get the comment ID from the URL
	commentID, err := app.readIDParam(r, "commentID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// validate url ID
	v := validator.New()
	if data.ValidateURLID(v, commentID, "comment_id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the reaction
	err = app.models.CommentManagerModel.DeleteReaction(app.contextGetUser(r).ID, commentID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "reaction successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

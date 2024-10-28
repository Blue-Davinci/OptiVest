package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"go.uber.org/zap"
)

// createNewFeedHandler() is a handler rresponsible for creating a new feed
// We will recieve a feed, validate it, and insert it
func (app *application) createNewFeedHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name            string `json:"name"`
		Url             string `json:"url"`
		ImgUrl          string `json:"img_url"`
		FeedType        string `json:"feed_type"`
		FeedCategory    string `json:"feed_category"`
		FeedDescription string `json:"feed_description"`
		IsHidden        bool   `json:"is_hidden"`
	}
	// read the request body
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// map the feed type to a constant
	feedType, err := app.models.FeedManager.MapFeedTypeToConstant(input.FeedType)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// create a new feed
	feed := &data.Feed{
		Name:            input.Name,
		URL:             input.Url,
		ImgUrl:          input.ImgUrl,
		FeedType:        feedType,
		FeedCategory:    input.FeedCategory,
		FeedDescription: input.FeedDescription,
		IsHidden:        input.IsHidden,
	}
	// validate
	v := validator.New()
	if data.ValidateFeed(v, feed); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// insert the feed
	err = app.models.FeedManager.CreateNewFeed(app.contextGetUser(r).ID, feed)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateFeed):
			v.AddError("url", "a feed with this URL already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"feed": feed}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateFeedHandler() is a handler responsible for updating a feed
// We will recieve a feed ID, validate it, and update it
func (app *application) updateFeedHandler(w http.ResponseWriter, r *http.Request) {
	// get the feed ID from the URL
	feedID, err := app.readIDParam(r, "feedID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	v := validator.New()
	// validate the feed ID
	if data.ValidateURLID(v, feedID, "id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// input struct
	var input struct {
		Name            *string `json:"name"`
		Url             *string `json:"url"`
		ImgUrl          *string `json:"img_url"`
		FeedType        *string `json:"feed_type"`
		FeedCategory    *string `json:"feed_category"`
		FeedDescription *string `json:"feed_description"`
		IsHidden        *bool   `json:"is_hidden"`
	}
	// read the request body
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get the feed
	feed, err := app.models.FeedManager.GetFeedByID(feedID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// get which fields have changed
	if input.Name != nil {
		feed.Name = *input.Name
	}
	if input.Url != nil {
		feed.URL = *input.Url
	}
	if input.ImgUrl != nil {
		feed.ImgUrl = *input.ImgUrl
	}
	if input.FeedType != nil {
		feedType, err := app.models.FeedManager.MapFeedTypeToConstant(*input.FeedType)
		if err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		feed.FeedType = feedType
	}
	if input.FeedCategory != nil {
		feed.FeedCategory = *input.FeedCategory
	}
	if input.FeedDescription != nil {
		feed.FeedDescription = *input.FeedDescription
	}
	if input.IsHidden != nil {
		feed.IsHidden = *input.IsHidden
	}
	// validate
	if data.ValidateFeed(v, feed); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the feed
	err = app.models.FeedManager.UpdateFeed(app.contextGetUser(r).ID, feed)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateFeed):
			v.AddError("url", "a feed with this URL already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"feed": feed}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// deleteFeedByIDHandler() is a handler responsible for deleting a feed by its ID
// We recieve the FeedID ffrom the URL, validate it and delete it
func (app *application) deleteFeedByIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the feed ID from the URL
	feedID, err := app.readIDParam(r, "feedID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	v := validator.New()
	// validate the feed ID
	if data.ValidateURLID(v, feedID, "id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the feed
	deletedID, err := app.models.FeedManager.DeleteFeedByID(app.contextGetUser(r).ID, feedID)
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
	message := fmt.Sprintf("feed with ID %d has been deleted", *deletedID)
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllRSSPostWithFavoriteTagsHandler() is a handler responsible for getting all RSS posts with favorite tags
// We will get all the posts, and send them back
func (app *application) getAllRSSPostWithFavoriteTagsHandler(w http.ResponseWriter, r *http.Request) {
	// make a struct to hold what we would want from the queries
	var input struct {
		Name          string
		FeedID        int
		IsEducational bool // this will be used to filter out educational posts
		data.Filters
	}
	//validate if queries are provided
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	// get our parameters
	input.Name = app.readString(qs, "name", "")                       // get our name parameter
	input.FeedID = app.readInt(qs, "feedID", 0, v)                    // get our feed_id parameter
	is_educational := app.readBoolean(qs, "is_educational", false, v) // get our is_educational parameter
	//get the page & pagesizes as ints and set to the embedded struct
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 18, v)
	// We don't use any sort for this endpoint
	input.Filters.Sort = app.readString(qs, "", "")
	// None of the sort values are supported for this endpoint
	input.Filters.SortSafelist = []string{"", ""}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get all the posts
	posts, metadata, err := app.models.FeedManager.GetAllRSSPostWithFavoriteTag(
		app.contextGetUser(r).ID,
		int64(input.FeedID),
		input.Name,
		app.postCategoryDecider(is_educational),
		input.Filters,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"metadata": metadata, "posts": posts}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

	notificationContent := data.NotificationContent{
		NotificationID: 1,
		Message:        "Congratulations! You have gotten your first posts!.",
		Meta: data.NotificationMeta{
			Url:      "http://localhost:5173/dashboard/notifications",
			ImageUrl: "https://images.unsplash.com/photo-1729429943621-90bc7c346e13?w=500&auto=format&fit=crop&q=60&ixlib=rb-4.0.3&ixid=M3wxMjA3fDB8MHxmZWF0dXJlZC1waG90b3MtZmVlZHw4fHx8ZW58MHx8fHx8",
			Tags:     "goal,completed",
		},
	}
	err = app.publishNotification(1, notificationContent)
	if err != nil {
		app.logger.Info("error publishing notification", zap.Error(err))
	}
}

// getRssFeedPostByIDHandler() is a handler responsible for getting a post by its ID
// We will recieve the post ID from the URL, validate it, and send it back
func (app *application) getRssFeedPostByIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the post ID from the URL
	postID, err := app.readIDParam(r, "postID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// validate the post ID
	v := validator.New()
	if data.ValidateURLID(v, postID, "postID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the post
	post, err := app.models.FeedManager.GetRssFeedPostByID(postID)
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
	err = app.writeJSON(w, http.StatusOK, envelope{"post": post}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewFavoriteOnPostHandler() is a handler responsible for creating a new favorite on a post
// We will recieve a post ID, validate it, and insert it
func (app *application) createNewFavoriteOnPostHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PostID int64 `json:"post_id"`
		FeedID int64 `json:"feed_id"`
	}
	// Read the JSON data from the request body
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// Create a new Favorite
	favorite := &data.RSSPostFavorite{
		PostID: input.PostID,
		FeedID: input.FeedID,
	}
	// Validate the Favorite
	v := validator.New()
	if data.ValidateRSSPostFavorite(v, favorite); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Insert the Favorite
	err = app.models.FeedManager.CreateNewFavoriteOnPost(app.contextGetUser(r).ID, favorite)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateFavorite):
			v.AddError("post_id", "this post is already favorited")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"favorite": favorite}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteFavoriteOnPostHandler() is a handler responsible for deleting a favorite on a post
// We will recieve a post ID, validate it, and delete it
func (app *application) deleteFavoriteOnPostHandler(w http.ResponseWriter, r *http.Request) {
	// Get the post ID from the URL
	postID, err := app.readIDParam(r, "postID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// Validate the post ID
	v := validator.New()
	if data.ValidateURLID(v, postID, "postID"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//app.logger.Info("postID", zap.Int64("postID", postID))
	// Delete the Favorite
	deletedID, err := app.models.FeedManager.DeleteFavoriteOnPost(app.contextGetUser(r).ID, postID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Send the response
	message := fmt.Sprintf("favorite with ID %d has been deleted", *deletedID)
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

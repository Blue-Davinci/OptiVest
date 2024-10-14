package main

import "net/http"

// getAllAwardsForUserByIDHandler() is an endpoint handler that returns all the awards for a user by ID
// we acquire the user id and retieve all the awards for that user
func (app *application) getAllAwardsForUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	// retrieve the user id from the context
	userID := app.contextGetUser(r).ID
	// get all the awards for a user by ID
	awards, err := app.models.AwardManager.GetAllAwardsForUserByID(userID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the awards as a JSON response
	err = app.writeJSON(w, http.StatusOK, envelope{"awards": awards}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis/v8"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var (
	ErrInvalidAuthentication = errors.New("invalid authentication token format")
	ErrNoDataFoundInRedis    = errors.New("no data found in Redis")
)

// Define an envelope type.
type envelope map[string]any

// Define a writeJSON() helper for sending responses. This takes the destination
// http.ResponseWriter, the HTTP status code to send, the data to encode to JSON, and a
// header map containing any additional HTTP headers we want to include in the response.
func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// Encode the data to JSON, returning the error if there was one.
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	// Append a newline to make it easier to view in terminal applications.
	js = append(js, '\n')
	// At this point, we know that we won't encounter any more errors before writing the
	// response, so it's safe to add any headers that we want to include.
	for key, value := range headers {
		w.Header()[key] = value
	}
	// Add the "Content-Type: application/json" header, then write the status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Use http.MaxBytesReader() to limit the size of the request body to 1MB.
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	// Initialize the json.Decoder, and call the DisallowUnknownFields() method on it
	// before decoding. This means that if the JSON from the client now includes any
	// field which cannot be mapped to the target destination, the decoder will return
	// an error instead of just ignoring the field.
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	// Decode the request body to the destination.
	err := dec.Decode(dst)
	err = app.jsonReadAndHandleError(err)
	if err != nil {
		return err
	}
	// Call Decode() again, using a pointer to an empty anonymous struct as the
	// destination. If the request body only contained a single JSON value this will
	// return an io.EOF error. So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

// Retrieve the "id" URL parameter from the current request context, then convert it to
// an integer and return it. If the operation isn't successful, return a nil UUID and an error.
func (app *application) readIDParam(r *http.Request, parameterName string) (int64, error) {
	// We use chi's URLParam method to get our ID parameter from the URL.
	params := chi.URLParam(r, parameterName)
	id, err := strconv.ParseInt(params, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid i-id parameter")
	}
	return id, nil
}

// jsonReadAndHandleError() is a helper function that takes an error as a parameter and
// returns a cleaned-up error message. This is used to provide more information in the
// event of a JSON decoding error.
func (app *application) jsonReadAndHandleError(err error) error {
	if err != nil {
		// Vars to carry our errors
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		// Add a new maxBytesError variable.
		var maxBytesError *http.MaxBytesError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown
		// field "<name>"". We check for this, extract the field name from the error,
		// and interpolate it into our custom error message. Note that there's an open
		// issue at https://github.com/golang/go/issues/29035 regarding turning this
		// into a distinct error type in the future.
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		// Use the errors.As() function to check whether the error has the type
		// *http.MaxBytesError. If it does, then it means the request body exceeded our
		// size limit of 1MB and we return a clear error message.
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}
	return nil
}

// isProfileComplete() checks whether all items have been filled in the user profile.
// If all items have been filled, it returns true, otherwise it returns false.
// This is a helper function to fill the ProfileCompleted field in the user struct.
func (app *application) isProfileComplete(user *data.User) bool {
	return user.FirstName != "" &&
		user.LastName != "" &&
		user.Email != "" &&
		user.PhoneNumber != "" &&
		user.DOB != (time.Time{}) &&
		user.Address != "" &&
		user.CountryCode != "" &&
		user.CurrencyCode != ""
}

// The background() helper accepts an arbitrary function as a parameter.
// It launches a background goroutine to execute the function.
// The done() method of the WaitGroup is called when the goroutine completes.
func (app *application) background(fn func()) {
	app.wg.Add(1)
	// Launch a background goroutine.
	go func() {
		//defer our done()
		defer app.wg.Done()
		// Recover any panic.
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error(fmt.Sprintf("%s", err))
			}
		}()
		// Execute the arbitrary function that we passed as the parameter.
		fn()
	}()
}

// saveCurrenciesToRedis() saves a currency list to REDIS  with the currency as the key
// and the rate as the value. Will be used in tandem with the currency conversion API.
func (app *application) saveCurrenciesToRedis(rates data.CurrencyRates) error {
	for currency, rate := range rates.ConversionRates {
		err := app.RedisDB.HSet(context.Background(), "currency_rates", currency, rate).Err()
		if err != nil {
			return data.ErrFailedToSaveRecordToRedis
		}
	}
	return nil
}

// getCurrenciesFromRedis retrieves all currency rates from Redis
func (app *application) getCurrenciesFromRedis() (data.CurrencyRates, error) {
	result, err := app.RedisDB.HGetAll(context.Background(), "currency_rates").Result()
	if err != nil {
		return data.CurrencyRates{}, fmt.Errorf("failed to get records from Redis: %w", err)
	}

	conversionRates := make(map[string]float64)
	for currency, rateStr := range result {
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil {
			return data.CurrencyRates{}, fmt.Errorf("failed to parse rate for currency %s: %w", currency, err)
		}
		conversionRates[currency] = rate
	}

	return data.CurrencyRates{ConversionRates: conversionRates}, nil
}

// verifyCurrencyInRedis() checks if a currency exists in REDIS. Will be used
// in tandem with the currency conversion API.
func (app *application) verifyCurrencyInRedis(currency string) error {
	exists, err := app.RedisDB.HExists(context.Background(), "currency_rates", currency).Result()
	if err != nil {
		return err
	}
	if !exists {
		return data.ErrFailedToGetCurrency
	}
	app.logger.Info("currency cerified, using cached currencies", zap.String("currency", currency))
	return nil
}

// getAndSaveAvailableCurrencies() gets the available currencies from the exchange rate API
func (app *application) getAndSaveAvailableCurrencies() error {
	url := fmt.Sprintf("%s/%s/latest/%s", app.config.api.apikeys.exchangerates.url,
		app.config.api.apikeys.exchangerates.key, app.config.api.defaultcurrency)
	currencies, err := GETRequest[data.CurrencyRates](app.http_client, url, nil)
	if err != nil {
		return err
	}
	// Save the currencies to Redis
	err = app.saveCurrenciesToRedis(currencies)
	if err != nil {
		return err
	}
	return nil
}

// The readString() helper returns a string value from the query string, or the provided
// default value if no matching key could be found.
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	// Extract the value for a given key from the query string. If no key exists this
	// will return the empty string "".
	s := qs.Get(key)
	// If no key exists (or the value is empty) then return the default value.
	if s == "" {
		return defaultValue
	}
	// Otherwise return the string.
	return s
}

// The readInt() helper reads a string value from the query string and converts it to an
// integer before returning. If no matching key could be found it returns the provided
// default value. If the value couldn't be converted to an integer, then we record an
// error message in the provided Validator instance.
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	// Extract the value from the query string.
	s := qs.Get(key)
	// If no key exists (or the value is empty) then return the default value.
	if s == "" {
		return defaultValue
	}
	// Try to convert the value to an int. If this fails, add an error message to the
	// validator instance and return the default value.
	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}
	// Otherwise, return the converted integer value.
	return i
}

// readFloat() reads a string value from the query string and converts it to a float64
// before returning. If no matching key could be found it returns the provided default value.
// If the value couldn't be converted to a float64, then we record an error message in the
// provided Validator instance.
func (app *application) readFloat64(qs url.Values, key string, defaultValue float64, v *validator.Validator) float64 {
	// Extract the value from the query string.
	s := qs.Get(key)
	// If no key exists (or the value is empty) then return the default value.
	if s == "" {
		return defaultValue
	}
	// Try to convert the value to a float64. If this fails, add an error message to the
	// validator instance and return the default value.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		v.AddError(key, "must be a float value")
		return defaultValue
	}
	// Otherwise, return the converted float64 value.
	return f
}

// readBoolean() reads a string value from the query string and converts it to a boolean
// before returning. If no matching key could be found it returns the provided default value.
// If the value couldn't be converted to a boolean, then we record an error message in the
// provided Validator instance.
func (app *application) readBoolean(qs url.Values, key string, defaultValue bool, v *validator.Validator) bool {
	// Extract the value from the query string.
	s := qs.Get(key)
	// If no key exists (or the value is empty) then return the default value.
	if s == "" {
		return defaultValue
	}
	// Try to convert the value to a boolean. If this fails, add an error message to the
	// validator instance and return the default value.
	b, err := strconv.ParseBool(s)
	if err != nil {
		v.AddError(key, "must be a boolean value")
		return defaultValue
	}
	// Otherwise, return the converted boolean value.
	return b
}

// readDate() reads a string value from the query string and converts it to a time.Time
// before returning. If no matching key could be found it returns the provided default value.
// If the value couldn't be converted to a time.Time, then we record an error message in the
// provided Validator instance.
func (app *application) readDate(qs url.Values, key string, defaultValue time.Time, v *validator.Validator) time.Time {
	// Extract the value from the query string.
	s := qs.Get(key)
	// If no key exists (or the value is empty) then return the default value.
	if s == "" {
		return defaultValue
	}
	// Try to convert the value to a time.Time. If this fails, add an error message to the
	// validator instance and return the default value.
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		v.AddError(key, "must be a date in the format YYYY-MM-DD")
		return defaultValue
	}
	// Otherwise, return the converted time.Time value.
	return d
}

// isLastDayOfMonth() checks if the given time is the last day of the month.
func (app *application) isLastDayOfMonth(t time.Time) bool {
	nextDay := t.AddDate(0, 0, 1) // Add one day
	return nextDay.Day() == 1     // If the next day is the first day of the month
}

// validateURL() checks if the input string is a valid URL
func validateURL(input string) error {
	parsedURL, err := url.ParseRequestURI(input)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Further validate URL components
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("URL must contain both scheme and host")
	}

	return nil
}

// calculateEstimatedPayoffDate() calculates the estimated payoff date for a debt
func (app *application) calculateEstimatedPayoffDate(debt *data.Debt) (time.Time, error) {
	if debt.MinimumPayment.LessThanOrEqual(decimal.NewFromFloat(0)) {
		return time.Time{}, errors.New("minimum payment cannot be zero or negative")
	}

	remainingBalance := debt.Amount
	interestRatePerMonth := debt.InterestRate.Div(decimal.NewFromInt(12)).Div(decimal.NewFromInt(100)) // Monthly interest
	months := 0

	// Loop through months and simulate payments until the balance is zero
	for remainingBalance.GreaterThan(decimal.NewFromFloat(0)) {
		// Add interest for the current month
		interest := remainingBalance.Mul(interestRatePerMonth)
		remainingBalance = remainingBalance.Add(interest)

		// Subtract the minimum payment
		remainingBalance = remainingBalance.Sub(debt.MinimumPayment)

		months++
	}

	// Calculate the estimated payoff date
	estimatedPayoffDate := debt.DueDate.AddDate(0, months, 0) // Add the number of months
	return estimatedPayoffDate, nil
}

// calculateInterestPayment calculates the interest payment for a debt
func (app *application) calculateInterestPayment(debt *data.Debt) (decimal.Decimal, error) {
	// Ensure debt is not nil
	if debt == nil {
		return decimal.Zero, errors.New("debt cannot be nil")
	}

	// Ensure interest rate is a valid value
	if debt.InterestRate.LessThan(decimal.NewFromFloat(0)) {
		return decimal.Zero, errors.New("interest rate cannot be negative")
	}

	// Calculate the duration in days since the last interest was calculated
	daysSinceLastCalculation := time.Since(debt.InterestLastCalculated).Hours() / 24
	if daysSinceLastCalculation < 0 {
		return decimal.Zero, errors.New("invalid calculation period")
	}

	// Convert annual interest rate to daily interest rate (assuming 365 days in a year)
	dailyInterestRate := debt.InterestRate.Div(decimal.NewFromFloat(365))

	// Calculate interest based on remaining balance and the number of days
	interestPayment := debt.RemainingBalance.Mul(dailyInterestRate).Mul(decimal.NewFromFloat(daysSinceLastCalculation))

	// Update the last calculated interest date to the current time
	debt.InterestLastCalculated = time.Now()

	return interestPayment, nil
}

// aunthenticatorHelper() is a helper function for the authentication middleware
// It takes in a request and returns a user and an error
// It retrieves the value of the Authorization header from the request. This will
// return the empty string "" if there is no such header found.
func (app *application) aunthenticatorHelper(r *http.Request) (*data.User, error) {
	// Retrieve the value of the Authorization header from the request. This will
	// return the empty string "" if there is no such header found.
	authorizationHeader := r.Header.Get("Authorization")
	// If there is no Authorization header found, use the contextSetUser() helper to
	// add the AnonymousUser to the request context. Then we
	// call the next handler in the chain and return without executing any of the
	// code below.
	if authorizationHeader == "" {
		return data.AnonymousUser, nil
	}
	// Otherwise, we expect the value of the Authorization header to be in the format
	// "Bearer <token>". We try to split this into its constituent parts, and if the
	// header isn't in the expected format we return a 401 Unauthorized response
	// using the invalidAuthenticationTokenResponse() helper
	headerParts := strings.Split(authorizationHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return nil, ErrInvalidAuthentication
	}
	// Extract the actual authentication token from the header parts.
	token := headerParts[1]
	//app.logger.Info("User id Connected", zap.String("Connected ID", token))
	// Validate the token to make sure it is in a sensible format.
	v := validator.New()
	// If the token isn't valid, use the invalidAuthenticationTokenResponse()
	// helper to send a response, rather than the failedValidationResponse() helper
	// that we'd normally use.
	if data.ValidateTokenPlaintext(v, token); !v.Valid() {
		return nil, ErrInvalidAuthentication
	}
	// Retrieve the details of the user associated with the authentication token,
	// again calling the invalidAuthenticationTokenResponse() helper if no
	// matching record was found. IMPORTANT: Notice that we are using
	// ScopeAuthentication as the first parameter here.
	user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token, app.config.encryption.key)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			return nil, ErrInvalidAuthentication
		default:
			return nil, ErrInvalidAuthentication
		}
	}
	return user, nil
}

// investmentTransactionValidatorHelper() is a helper validation function for the investment transaction handler
// We take in a user ID and a *transaction struct. We extract the investmentID, from there we get the investment type
// Depending on that investment type i.e (stock,bond,alternative), we check if that ID exists for that user
// in the respective table
// If it does not exist, we add an error to the validator
func (app *application) investmentTransactionValidatorHelper(v *validator.Validator, transaction *data.InvestmentTransaction) interface{} {
	var investment interface{}
	// check if the investment exists
	switch transaction.InvestmentType {
	case data.InvPortInvestmentTypeStock:
		stock, err := app.models.InvestmentPortfolioManager.GetStockByStockID(transaction.InvestmentID)
		if err != nil {
			v.AddError("investment_id", "stock investment does not exist")
		}
		investment = stock
	case data.InvPortInvestmentTypeBond:
		bond, err := app.models.InvestmentPortfolioManager.GetBondByBondID(transaction.InvestmentID)
		if err != nil {
			v.AddError("investment_id", "bond investment does not exist")
		}
		investment = bond
	case data.InvPortInvestmentTypeAlternative:
		alternative, err := app.models.InvestmentPortfolioManager.GetAlternativeInvestmentByAlternativeID(transaction.InvestmentID)
		if err != nil {
			v.AddError("investment_id", "alternative investment does not exist")
		}
		investment = alternative
	default:
		v.AddError("investment_type", "invalid investment type")
		investment = nil
	}
	return investment
}

// updateInvestmentTransactionHelper() This will update the relevant table record when there is a new transaction
// when a buy or sell is made, we update the quantity and the current value of the investment
// we need to recieve the type of transaction and an interface which we need to cast to the correct type
// Then all we update is the quantity, if transaction type is sell, we substract the quantity
// if transaction type is buy, we add the quantity. Each unique investment type will have its own
// case for this function in terms of updates i.e stock, bond and alternative
func (app *application) updateInvestmentTransactionHelper(userID int64, transactionType string, transactionQuantity decimal.Decimal, investment interface{}) error {
	switch transactionType {
	case "buy":
		switch t := investment.(type) {
		case *data.StockInvestment:
			t.Quantity = t.Quantity.Add(transactionQuantity)
			// update passing the fully updated struct
			err := app.models.InvestmentPortfolioManager.UpdateStockInvestment(userID, t)
			if err != nil {
				return err
			}
		case *data.BondInvestment:
			t.Quantity = t.Quantity.Add(transactionQuantity)
			// update passing the fully updated struct
			err := app.models.InvestmentPortfolioManager.UpdateBondInvestment(userID, t)
			if err != nil {
				return err
			}
		case *data.AlternativeInvestment:
			t.Quantity = t.Quantity.Add(transactionQuantity)
			// update passing the fully updated struct
			err := app.models.InvestmentPortfolioManager.UpdateAlternativeInvestment(userID, t)
			if err != nil {
				return err
			}
		}
	case "sell":
		switch t := investment.(type) {
		case *data.StockInvestment:
			t.Quantity = t.Quantity.Sub(transactionQuantity)
			// update passing the fully updated struct
			err := app.models.InvestmentPortfolioManager.UpdateStockInvestment(userID, t)
			if err != nil {
				return err
			}
		case *data.BondInvestment:
			t.Quantity = t.Quantity.Sub(transactionQuantity)
			// update passing the fully updated struct
			err := app.models.InvestmentPortfolioManager.UpdateBondInvestment(userID, t)
			if err != nil {
				return err
			}
		case *data.AlternativeInvestment:
			t.Quantity = t.Quantity.Sub(transactionQuantity)
			// update passing the fully updated struct
			err := app.models.InvestmentPortfolioManager.UpdateAlternativeInvestment(userID, t)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Generic method to get data from Redis and unmarshal into the desired type
func getFromCache[T any](ctx context.Context, rdb *redis.Client, key string) (*T, error) {
	cachedData, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrNoDataFoundInRedis // Data not found
	} else if err != nil {
		return nil, err // Some other error
	}

	var result T
	err = json.Unmarshal([]byte(cachedData), &result)
	if err != nil {
		return nil, err // Error unmarshalling data
	}

	return &result, nil
}

// setToCache() is a Generic method to cache data in Redis with a TTL
func setToCache[T any](ctx context.Context, rdb *redis.Client, key string, value *T, ttl time.Duration) error {
	// Marshal the value into JSON
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Set the value in Redis with TTL
	// 0 ttl means no expiration
	err = rdb.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return err
	}

	return nil
}

// calculateYearsToMaturity() calculates the number of years to maturity for a bond
func (app *application) calculateYearsToMaturity(bondMD data.CustomTime1) int {
	yearsToMaturity := int(time.Until(bondMD.ToTime()).Hours() / (24 * 365))
	return yearsToMaturity
}

func (app *application) postCategoryDecider(isEducational bool) string {
	if isEducational {
		return "finance education"
	}
	return "finance"
}

// processOCRRequestHelper() is a helper function that will process the OCR request
// We will send a POST request to the OCR.Space API endpoint to get the text from the image
// We will then return the OCRResponse
func (app *application) proces1sOCRRequestHelper(url string) (*data.OCRResponse, error) {
	// we need a form Body for this, so we create a form body
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	// Add URL
	err := writer.WriteField("url", url)
	if err != nil {
		return nil, err
	}
	// Add necessary fields for OCR engine 2 and other options
	fields := map[string]string{
		"language":                     "eng",
		"isOverlayRequired":            "false",
		"OCREngine":                    "2",
		"isCreateSearchablePdf":        "false",
		"isSearchablePdfHideTextLayer": "false",
	}
	// Add all fields to the form-data
	for key, value := range fields {
		err := writer.WriteField(key, value)
		if err != nil {
			return nil, err
		}
	}
	// Close the writer
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	// set apikey header
	headers := map[string]string{
		"apikey":       app.config.api.apikeys.ocrspace.key,
		"Content-Type": writer.FormDataContentType(),
	}
	// print the body
	//app.logger.Info(requestBody.String())
	// call our POSTREQUEST http client with OCRResponse
	response, err := POSTRequest[data.OCRResponse](
		app.http_client,
		app.config.api.apikeys.ocrspace.url,
		headers,
		requestBody,
		true,
	)
	if err != nil {
		return nil, err
	}
	// print api key and url used
	//app.logger.Info("ITEMS USED", zap.String("url", app.config.api.apikeys.ocrspace.url), zap.String("API Key", app.config.api.apikeys.ocrspace.key))
	//app.logger.Info("OCR Response", zap.Any("response", response))
	app.logger.Info("Done processing OCR request successfully")
	return &response, nil
}

// notificationPreperationHelper() is a helper function that will prepare the notification
// we will recieve the , []mesage,notificationtype , url, imgurl and tags
// for each item, we will make a notificationcontent struct and use app.PublishNotificationToRedis
// to publish the notification to Redis, passing in the userID, notification type and the notification content
func (app *application) notificationPreperationHelper(userID int64, messages []string, notificationType, url, imgURL, tags string) error {
	if len(messages) == 0 {
		return nil
	}
	for _, message := range messages {
		notificationContent := data.NotificationContent{
			Message: message,
			Meta: data.NotificationMeta{
				Url:      url,
				ImageUrl: imgURL,
				Tags:     tags,
			},
		}
		err := app.PublishNotificationToRedis(userID, notificationType, notificationContent)
		if err != nil {
			return err
		}
	}
	return nil
}

// groupAndNestComments nests and sorts comments into a tree structure
func (app *application) groupAndNestComments(comments []*data.EnrichedComment) map[int64]*data.OrganizedComment {
	// Create a map to group comments by parent ID
	groupedComments := make(map[int64][]*data.EnrichedComment)
	for _, comment := range comments {
		groupedComments[comment.Comment.ParentID] = append(groupedComments[comment.Comment.ParentID], comment)
	}

	organized := make(map[int64]*data.OrganizedComment)

	// Process parent comments
	for _, parent := range groupedComments[0] { // Parent comments have ParentID = 0
		organized[parent.Comment.ID] = &data.OrganizedComment{
			Parent:  parent,
			Replies: []*data.EnrichedComment{},
		}
	}

	// Attach replies to their respective parents
	for parentID, replies := range groupedComments {
		if parentID == 0 {
			continue // Skip parent comments
		}
		if parent, exists := organized[parentID]; exists {
			// Sort replies for each parent by CreatedAt in descending order
			sort.Slice(replies, func(i, j int) bool {
				return replies[i].Comment.CreatedAt.After(replies[j].Comment.CreatedAt)
			})
			parent.Replies = replies
		}
	}

	return organized
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (app *application) returnFormattedRedisKeys(key string, userID int64) string {
	return fmt.Sprintf("%s:%d", key, userID)
}

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

// saveAndUpdateSurplus() this method will be responsible for saving and updating the surplus
// of a user's nudget. It will be called after a budget has been created or updated as well as
// when a goal is added or updated.
// When called we get the new total surplus amount from the caller.
//
//	we check if the the data.RedisFinManSurplusPrefix:userid:budgetid key exists in Redis.
//
// if it does, we update the surplus amount and return nil error.
// Otherwise, we save the surplus amount to Redis and return nil as an error.
func (app *application) saveAndUpdateSurplus(userID, budgetID int64, surplus decimal.Decimal) error {
	// Generate Redis key for budget surplus
	budgetSurpluskey := fmt.Sprintf("%s:%d", data.RedisFinManSurplusPrefix, userID)
	key := app.returnFormattedRedisKeys(budgetSurpluskey, budgetID)

	// Use HSet to update or insert the surplus value
	err := app.RedisDB.HSet(context.Background(), key, "surplus", surplus.String()).Err()
	if err != nil {
		return err
	}

	// Set a TTL of 24 hours for the surplus key
	err = app.RedisDB.Expire(context.Background(), key, 24*time.Hour).Err()
	if err != nil {
		return err
	}

	return nil
}

// getSurplus() retrieves the surplus for a budget from Redis. If the surplus does not exist,
// it returns a zero value and nil error. If there are any other issues, it returns an error.
func (app *application) getSurplus(userID, budgetID int64) (decimal.Decimal, error) {
	// Generate Redis key for budget surplus
	budgetSurplusKey := fmt.Sprintf("%s:%d", data.RedisFinManSurplusPrefix, userID)
	key := app.returnFormattedRedisKeys(budgetSurplusKey, budgetID)

	// Retrieve the surplus from Redis
	surplusStr, err := app.RedisDB.HGet(context.Background(), key, "surplus").Result()
	if err != nil {
		// Handle error: return 0 and error if key does not exist or other issues
		if err == redis.Nil {
			return decimal.Decimal{}, nil // Key does not exist, return zero value
		}
		return decimal.Decimal{}, err // Return any other errors
	}

	// Convert the surplus string to decimal.Decimal
	surplus, err := decimal.NewFromString(surplusStr)
	if err != nil {
		return decimal.Decimal{}, err // Return error if conversion fails
	}

	return surplus, nil // Return the retrieved surplus
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

// isLastDayOfMonth() checks if the given time is the last day of the month.
func (app *application) isLastDayOfMonth(t time.Time) bool {
	nextDay := t.AddDate(0, 0, 1) // Add one day
	return nextDay.Day() == 1     // If the next day is the first day of the month
}

// cacheData caches any data structure with a given key and TTL
func (app *application) cacheSerializedData(key string, data interface{}, ttl time.Duration) error {
	serializedData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = app.RedisDB.Set(context.Background(), key, serializedData, ttl).Err()
	if err != nil {
		return err
	}
	app.logger.Info("saving data to cache", zap.String("key", key))
	return nil
}

// getCachedData retrieves cached data for a given key and deserializes it into the provided target
func (app *application) getSerializedCachedData(key string, target interface{}) (bool, error) {
	cachedData, err := app.RedisDB.Get(context.Background(), key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}

	err = json.Unmarshal(cachedData, target)
	if err != nil {
		return false, err
	}

	app.logger.Info("retrieved cached data", zap.String("key", key))
	return true, nil
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

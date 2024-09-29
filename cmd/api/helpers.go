package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
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
	app.logger.Info("Currency exists in Redis", zap.String("currency", currency))
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

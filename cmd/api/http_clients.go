package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// Client represents the central HTTP client with retry capabilities
type Optivet_Client struct {
	httpClient *retryablehttp.Client
}

// NewClient initializes and returns a new Client with custom configurations
func NewClient(timeout time.Duration, retries int) *Optivet_Client {
	// Create a retryable HTTP client with custom settings
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = retries
	retryClient.HTTPClient.Timeout = timeout
	retryClient.Backoff = retryablehttp.LinearJitterBackoff
	retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler
	retryClient.Logger = nil

	return &Optivet_Client{
		httpClient: retryClient,
	}
}

// GETRequest sends a GET request to the specified URL and unmarshals the response into a generic type T
func GETRequest[T any](c *Optivet_Client, url string, headers map[string]string) (T, error) {
	var result T

	// Create a new request
	req, err := retryablehttp.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	// Check if the response status is not 2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, errors.New("non-2xx response code")
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	// Unmarshal the response into the provided generic type
	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}

// POSTRequest sends a POST request with a body to the specified URL and unmarshals the response into a generic type T
func POSTRequest[T any](c *Optivet_Client, url string, headers map[string]string, body interface{}) (T, error) {
	var result T

	// Marshal the body to JSON
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return result, err
	}

	// Create a new POST request
	req, err := retryablehttp.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return result, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	// Check if the response status is not 2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, errors.New("non-2xx response code")
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	// Unmarshal the response into the provided generic type
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}

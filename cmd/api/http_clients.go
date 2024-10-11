package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
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
	//fmt.Printf("Response: %v", result)

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
func (app *application) LLMRequest(url string, headers map[string]string, body string) (string, error) {
	// Convert the body to bytes directly without marshaling again
	jsonBody := []byte(body)
	clientC := NewClient(30*time.Second, 3)

	// Create a new POST request using retryablehttp
	req, err := retryablehttp.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	resp, err := clientC.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check if the response status is not 2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		app.logger.Info("Non-2xx response code", zap.String("status", resp.Status))
		return "", errors.New("non-2xx response code")
	}

	// Variable to accumulate the entire response
	var fullResponse string

	// Read the response stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and headers
		if line == "" || line == "data: " {
			continue
		}

		// Remove the "data: " prefix
		line = line[6:]

		// Parse the chunk as JSON
		var chunk Chunk
		err := json.Unmarshal([]byte(line), &chunk)
		if err != nil {
			fmt.Println("Error parsing chunk:", err)
			continue
		}

		// Accumulate the content part of the chunk into the fullResponse
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullResponse += choice.Delta.Content
			}
		}

		// Stop reading if we hit the finish reason
		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil {
				fmt.Println("\nFinished")
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return fullResponse, nil
}

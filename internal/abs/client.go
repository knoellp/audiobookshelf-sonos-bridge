package abs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
	// DefaultConnectTimeout is the default connection timeout.
	DefaultConnectTimeout = 10 * time.Second
	// DefaultMaxRetries is the default number of retry attempts.
	DefaultMaxRetries = 2
	// DefaultRetryDelay is the base delay between retries.
	DefaultRetryDelay = 500 * time.Millisecond
)

// Client is an HTTP client for the Audiobookshelf API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewClient creates a new Audiobookshelf client with sensible defaults.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   DefaultConnectTimeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		maxRetries: DefaultMaxRetries,
		retryDelay: DefaultRetryDelay,
	}
}

// WithToken returns a copy of the client with the given token set.
func (c *Client) WithToken(token string) *Client {
	return &Client{
		baseURL:    c.baseURL,
		token:      token,
		httpClient: c.httpClient,
		maxRetries: c.maxRetries,
		retryDelay: c.retryDelay,
	}
}

// WithTimeout returns a copy of the client with the given timeout.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	return &Client{
		baseURL: c.baseURL,
		token:   c.token,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   min(timeout/3, DefaultConnectTimeout),
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: timeout / 2,
			},
		},
		maxRetries: c.maxRetries,
		retryDelay: c.retryDelay,
	}
}

// WithRetries returns a copy of the client with custom retry settings.
func (c *Client) WithRetries(maxRetries int, retryDelay time.Duration) *Client {
	return &Client{
		baseURL:    c.baseURL,
		token:      c.token,
		httpClient: c.httpClient,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// min returns the smaller of two durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// Login authenticates with Audiobookshelf and returns the user with token.
func (c *Client) Login(ctx context.Context, username, password string) (*User, error) {
	req := LoginRequest{
		Username: username,
		Password: password,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrInvalidCredentials
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var loginResp struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	return &loginResp.User, nil
}

// GetLibraries returns all libraries accessible to the user.
func (c *Client) GetLibraries(ctx context.Context) ([]Library, error) {
	var resp LibrariesResponse
	if err := c.get(ctx, "/api/libraries", &resp); err != nil {
		return nil, err
	}
	return resp.Libraries, nil
}

// GetLibraryItems returns items from a library with optional filtering.
func (c *Client) GetLibraryItems(ctx context.Context, libraryID string, opts ItemsOptions) (*ItemsResponse, error) {
	path := fmt.Sprintf("/api/libraries/%s/items", libraryID)
	query := opts.ToQuery()
	if query != "" {
		path += "?" + query
	}

	var resp ItemsResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFilterData returns filter data for a library.
func (c *Client) GetFilterData(ctx context.Context, libraryID string) (*FilterData, error) {
	path := fmt.Sprintf("/api/libraries/%s/filterdata", libraryID)

	var resp FilterData
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetItem returns a single library item by ID.
func (c *Client) GetItem(ctx context.Context, itemID string) (*LibraryItem, error) {
	path := fmt.Sprintf("/api/items/%s", itemID)

	var resp LibraryItem
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetProgress returns the playback progress for an item.
func (c *Client) GetProgress(ctx context.Context, itemID string) (*Progress, error) {
	path := fmt.Sprintf("/api/me/progress/%s", itemID)

	var resp Progress
	if err := c.get(ctx, path, &resp); err != nil {
		// 404 means no progress yet, return empty progress
		if err == ErrNotFound {
			return &Progress{
				LibraryItemID: itemID,
				CurrentTime:   0,
				Progress:      0,
			}, nil
		}
		return nil, err
	}
	return &resp, nil
}

// UpdateProgress updates the playback progress for an item.
func (c *Client) UpdateProgress(ctx context.Context, itemID string, update ProgressUpdate) error {
	path := fmt.Sprintf("/api/me/progress/%s", itemID)

	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal progress update: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("progress update request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// GetCover proxies a cover image request and returns the response body.
func (c *Client) GetCover(ctx context.Context, itemID string) (io.ReadCloser, string, error) {
	path := fmt.Sprintf("/api/items/%s/cover", itemID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("cover request failed: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, "", ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	return resp.Body, contentType, nil
}

// get performs a GET request and decodes the JSON response with retry logic.
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryDelay * time.Duration(attempt)):
			}
		}

		err := c.doGet(ctx, path, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on context cancellation or non-retryable errors
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !isRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// doGet performs a single GET request.
func (c *Client) doGet(ctx context.Context, path string, result interface{}) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	// Server errors are retryable
	if resp.StatusCode >= 500 {
		return &serverError{StatusCode: resp.StatusCode}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// serverError represents a server-side error.
type serverError struct {
	StatusCode int
}

func (e *serverError) Error() string {
	return fmt.Sprintf("server error: %d", e.StatusCode)
}

// isRetryableError checks if an error is transient and should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry authentication errors
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrInvalidCredentials) {
		return false
	}

	// Don't retry not found
	if errors.Is(err, ErrNotFound) {
		return false
	}

	// Retry server errors
	var srvErr *serverError
	if errors.As(err, &srvErr) {
		return true
	}

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Connection errors are retryable
	errStr := err.Error()
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"no route to host",
		"network is unreachable",
		"i/o timeout",
		"EOF",
	}
	for _, msg := range retryableMessages {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(msg)) {
			return true
		}
	}

	return false
}

// ItemsOptions configures the items query.
type ItemsOptions struct {
	Limit   int
	Page    int
	Sort    string
	Desc    bool
	Filter  string
	Search  string
	Include string
}

// ToQuery converts options to URL query string.
func (o ItemsOptions) ToQuery() string {
	params := url.Values{}

	if o.Limit > 0 {
		params.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Page > 0 {
		params.Set("page", strconv.Itoa(o.Page))
	}
	if o.Sort != "" {
		params.Set("sort", o.Sort)
	}
	if o.Desc {
		params.Set("desc", "1")
	}
	// Note: Filter and Search both use the "filter" query parameter.
	// If both are set, Search takes precedence (it's a search filter type).
	// Audiobookshelf filter format: filter.<type>.<value>
	if o.Search != "" {
		// Search is a special filter type that takes precedence
		params.Set("filter", "search."+url.QueryEscape(o.Search))
	} else if o.Filter != "" {
		params.Set("filter", o.Filter)
	}
	if o.Include != "" {
		params.Set("include", o.Include)
	}

	return params.Encode()
}

// Errors
var (
	ErrInvalidCredentials = fmt.Errorf("invalid credentials")
	ErrUnauthorized       = fmt.Errorf("unauthorized")
	ErrNotFound           = fmt.Errorf("not found")
)

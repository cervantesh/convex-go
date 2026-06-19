package convex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Query executes a Convex query function.
func (c *HTTPClient) Query(ctx context.Context, path string, args any) (any, error) {
	value, err := c.QueryValue(ctx, path, args)
	if err != nil {
		return nil, err
	}
	return value.GoValue(), nil
}

// Mutation executes a Convex mutation function.
func (c *HTTPClient) Mutation(ctx context.Context, path string, args any, opts ...MutationOption) (any, error) {
	value, err := c.MutationValue(ctx, path, args, opts...)
	if err != nil {
		return nil, err
	}
	return value.GoValue(), nil
}

// Action executes a Convex action function.
func (c *HTTPClient) Action(ctx context.Context, path string, args any) (any, error) {
	value, err := c.ActionValue(ctx, path, args)
	if err != nil {
		return nil, err
	}
	return value.GoValue(), nil
}

// QueryInto executes a query and decodes the result into out using encoding/json.
func (c *HTTPClient) QueryInto(ctx context.Context, path string, args any, out any) error {
	result, err := c.Query(ctx, path, args)
	if err != nil {
		return err
	}
	return decodeInto(result, out)
}

// MutationInto executes a mutation and decodes the result into out using encoding/json.
func (c *HTTPClient) MutationInto(ctx context.Context, path string, args any, out any, opts ...MutationOption) error {
	result, err := c.Mutation(ctx, path, args, opts...)
	if err != nil {
		return err
	}
	return decodeInto(result, out)
}

// ActionInto executes an action and decodes the result into out using encoding/json.
func (c *HTTPClient) ActionInto(ctx context.Context, path string, args any, out any) error {
	result, err := c.Action(ctx, path, args)
	if err != nil {
		return err
	}
	return decodeInto(result, out)
}

type functionRequest struct {
	Path   string `json:"path"`
	Format string `json:"format"`
	Args   []any  `json:"args"`
}

type anyFunctionRequest struct {
	ComponentPath string `json:"componentPath,omitempty"`
	Path          string `json:"path"`
	Format        string `json:"format"`
	Args          any    `json:"args"`
}

type timestampResponse struct {
	Timestamp string `json:"ts"`
}

type functionResponse struct {
	Status       string          `json:"status"`
	Value        json.RawMessage `json:"value"`
	ErrorMessage string          `json:"errorMessage"`
	ErrorData    json.RawMessage `json:"errorData"`
	LogLines     []string        `json:"logLines"`
}

// QueryValue executes a query and returns a typed Convex Value.
func (c *HTTPClient) QueryValue(ctx context.Context, path string, args any) (Value, error) {
	return c.call(ctx, QueryKind, path, args, "")
}

// MutationValue executes a mutation and returns a typed Convex Value.
func (c *HTTPClient) MutationValue(ctx context.Context, path string, args any, opts ...MutationOption) (Value, error) {
	options := mutationOptionsFrom(opts...)
	if !options.skipQueue {
		release, err := c.acquireMutationTurn(ctx)
		if err != nil {
			return Value{}, err
		}
		if release == nil {
			return Value{}, fmt.Errorf("convex: mutation queue release missing")
		}
		defer release()
	}
	return c.call(ctx, MutationKind, path, args, "")
}

func (c *HTTPClient) acquireMutationTurn(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.mutationQueue == nil {
		return nil, fmt.Errorf("convex: uninitialized HTTP client")
	}
	if cap(c.mutationQueue) == 0 {
		return nil, fmt.Errorf("convex: unready mutation queue")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.mutationQueue:
		return func() {
			c.mutationQueue <- struct{}{}
		}, nil
	}
}

// ActionValue executes an action and returns a typed Convex Value.
func (c *HTTPClient) ActionValue(ctx context.Context, path string, args any) (Value, error) {
	return c.call(ctx, ActionKind, path, args, "")
}

// Function executes a Convex function of unknown type through /api/function.
func (c *HTTPClient) Function(ctx context.Context, path string, args any, componentPath string) (any, error) {
	value, err := c.FunctionValue(ctx, path, args, componentPath)
	if err != nil {
		return nil, err
	}
	return value.GoValue(), nil
}

// FunctionValue executes a Convex function of unknown type through /api/function.
func (c *HTTPClient) FunctionValue(ctx context.Context, path string, args any, componentPath string) (Value, error) {
	return c.callAnyFunction(ctx, path, args, componentPath)
}

// GetTimestamp returns a timestamp token for consistent reads.
func (c *HTTPClient) GetTimestamp(ctx context.Context) (string, error) {
	resp, err := c.doRequestWithAuthRetry(ctx, http.MethodPost, c.address+"/api/query_ts", nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	var decoded timestampResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("convex: failed to decode timestamp response: %w", err)
	}
	return decoded.Timestamp, nil
}

// QueryAtTimestamp executes a query at a timestamp returned by GetTimestamp.
func (c *HTTPClient) QueryAtTimestamp(ctx context.Context, path string, args any, timestamp string) (Value, error) {
	if strings.TrimSpace(timestamp) == "" {
		return Value{}, fmt.Errorf("convex: timestamp cannot be empty")
	}
	return c.call(ctx, QueryKind, path, args, timestamp)
}

// ConsistentQuery obtains a timestamp and executes the query at that timestamp.
func (c *HTTPClient) ConsistentQuery(ctx context.Context, path string, args any) (Value, error) {
	timestamp, err := c.GetTimestamp(ctx)
	if err != nil {
		return Value{}, err
	}
	return c.QueryAtTimestamp(ctx, path, args, timestamp)
}

func (c *HTTPClient) call(ctx context.Context, kind FunctionKind, path string, args any, timestamp string) (Value, error) {
	if strings.TrimSpace(path) == "" {
		return Value{}, fmt.Errorf("convex: function path cannot be empty")
	}
	encodedArgs, err := encodeArgs(args)
	if err != nil {
		return Value{}, err
	}
	payload := functionRequest{
		Path:   path,
		Format: "convex_encoded_json",
		Args:   []any{encodedArgs},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Value{}, err
	}

	endpoint := c.address + "/api/" + string(kind)
	if kind == QueryKind && timestamp != "" {
		endpoint = c.address + "/api/query_at_ts"
		body, err = json.Marshal(map[string]any{
			"path":   path,
			"format": "convex_encoded_json",
			"args":   []any{encodedArgs},
			"ts":     timestamp,
		})
		if err != nil {
			return Value{}, err
		}
	}
	return c.doFunctionRequest(ctx, kind, path, endpoint, body)
}

func (c *HTTPClient) callAnyFunction(ctx context.Context, path string, args any, componentPath string) (Value, error) {
	if strings.TrimSpace(path) == "" {
		return Value{}, fmt.Errorf("convex: function path cannot be empty")
	}
	encodedArgs, err := encodeArgs(args)
	if err != nil {
		return Value{}, err
	}
	payload := anyFunctionRequest{
		ComponentPath: componentPath,
		Path:          path,
		Format:        "convex_encoded_json",
		Args:          encodedArgs,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Value{}, err
	}
	return c.doFunctionRequest(ctx, FunctionKind("function"), path, c.address+"/api/function", body)
}

func (c *HTTPClient) doFunctionRequest(ctx context.Context, kind FunctionKind, path string, endpoint string, body []byte) (Value, error) {
	resp, err := c.doRequestWithAuthRetry(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return Value{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Value{}, err
	}
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && resp.StatusCode != statusCodeUDFFailed {
		return Value{}, &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
	}

	var decoded functionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return Value{}, fmt.Errorf("convex: failed to decode response: %w", err)
	}
	switch decoded.Status {
	case "success":
		return ParseValueJSON(decoded.Value)
	case "error":
		functionErr := &FunctionError{
			Kind:        kind,
			Path:        path,
			Message:     decoded.ErrorMessage,
			LogLines:    decoded.LogLines,
			StatusCode:  resp.StatusCode,
			RawResponse: string(respBody),
		}
		if len(decoded.ErrorData) != 0 {
			data, err := ParseValueJSON(decoded.ErrorData)
			if err != nil {
				return Value{}, fmt.Errorf("convex: failed to decode error data: %w", err)
			}
			functionErr.DataValue = data
			functionErr.Data = data.GoValue()
			functionErr.Convex = &ConvexError{
				Message: decoded.ErrorMessage,
				Data:    data,
			}
			functionErr.HasData = true
		}
		return Value{}, functionErr
	default:
		return Value{}, fmt.Errorf("convex: invalid response status %q", decoded.Status)
	}
}

func (c *HTTPClient) setAuthCallback(fetcher UserTokenFetcher) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authCallback = fetcher
	c.auth = ""
	c.adminAuth = ""
}

func (c *HTTPClient) doRequestWithAuthRetry(ctx context.Context, method string, endpoint string, body []byte) (*http.Response, error) {
	resp, usedCallback, err := c.doAuthenticatedRequest(ctx, method, endpoint, body, false)
	if err != nil {
		return nil, err
	}
	if !usedCallback || (resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden) {
		return resp, nil
	}
	_ = resp.Body.Close()
	resp, _, err = c.doAuthenticatedRequest(ctx, method, endpoint, body, true)
	return resp, err
}

func (c *HTTPClient) doAuthenticatedRequest(ctx context.Context, method string, endpoint string, body []byte, forceRefresh bool) (*http.Response, bool, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Convex-Client", c.clientID)
	authHeader, usedCallback, err := c.authHeaderValue(forceRefresh)
	if err != nil {
		return nil, usedCallback, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, usedCallback, err
	}
	return resp, usedCallback, nil
}

func (c *HTTPClient) authHeaderValue(forceRefresh bool) (string, bool, error) {
	c.mu.RLock()
	adminAuth := c.adminAuth
	auth := c.auth
	authCallback := c.authCallback
	c.mu.RUnlock()
	if authCallback != nil {
		token, err := authCallback(forceRefresh)
		if err != nil {
			return "", true, err
		}
		if token == "" {
			return "", true, nil
		}
		return "Bearer " + token, true, nil
	}
	if adminAuth != "" {
		return "Convex " + adminAuth, false, nil
	}
	if auth != "" {
		return "Bearer " + auth, false, nil
	}
	return "", false, nil
}

func decodeInto(result any, out any) error {
	if out == nil {
		return fmt.Errorf("convex: nil output target")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

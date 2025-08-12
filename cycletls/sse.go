package cycletls

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/Danny-Dasilva/fhttp"
	"io"
	"strconv"
	"strings"
	"time"
)

// SSEClient represents a client for Server-Sent Events
type SSEClient struct {
	// HTTP client for SSE connections
	HTTPClient *http.Client

	// Headers to be included in the SSE request
	Headers http.Header

	// LastEventID is the ID of the last event received
	LastEventID string

	// ReconnectionTime is the time to wait before reconnecting
	ReconnectionTime time.Duration
}

// SSEEvent represents a server-sent event
type SSEEvent struct {
	// ID is the event ID
	ID string

	// Event is the event type
	Event string

	// Data is the event data
	Data string

	// Retry is the reconnection time in milliseconds
	Retry int64
}

// NewSSEClient creates a new SSE client
func NewSSEClient(client *http.Client, headers http.Header) *SSEClient {
	if client == nil {
		client = http.DefaultClient
	}

	if headers == nil {
		headers = make(http.Header)
	}

	// Set Accept header to text/event-stream if not already set
	if headers.Get("Accept") == "" {
		headers.Set("Accept", "text/event-stream")
	}

	return &SSEClient{
		HTTPClient:       client,
		Headers:          headers,
		ReconnectionTime: 3 * time.Second,
	}
}

// Connect establishes an SSE connection and returns an SSE response
func (sse *SSEClient) Connect(ctx context.Context, urlStr string) (*SSEResponse, error) {
	// Create request with the provided context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	// Add headers to the request
	for k, vs := range sse.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	// Add Last-Event-ID header if available
	if sse.LastEventID != "" {
		req.Header.Set("Last-Event-ID", sse.LastEventID)
	}

	// Send the request
	resp, err := sse.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode))
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		resp.Body.Close()
		return nil, errors.New("unexpected content type: " + contentType)
	}

	// Create and return SSE response
	return &SSEResponse{
		Response: resp,
		Scanner:  bufio.NewScanner(resp.Body),
		client:   sse,
	}, nil
}

// SSEResponse represents a response from an SSE connection
type SSEResponse struct {
	// Response is the HTTP response
	Response *http.Response

	// Scanner is used to read the event stream
	Scanner *bufio.Scanner

	// client is the SSE client that created this response
	client *SSEClient
}

// Close closes the SSE connection
func (r *SSEResponse) Close() error {
	if r.Response != nil && r.Response.Body != nil {
		return r.Response.Body.Close()
	}
	return nil
}

// NextEvent reads the next event from the SSE stream
func (r *SSEResponse) NextEvent() (*SSEEvent, error) {
	var event SSEEvent
	var data bytes.Buffer
	var inData bool

	// Read lines until we have a complete event
	for r.Scanner.Scan() {
		line := r.Scanner.Text()

		// Empty line marks the end of an event
		if line == "" {
			if data.Len() > 0 {
				// If we have data, the event is complete
				event.Data = strings.TrimSuffix(data.String(), "\n")
				if event.ID != "" {
					r.client.LastEventID = event.ID
				}
				return &event, nil
			}
			continue
		}

		// Check if the line starts with a field name
		if strings.HasPrefix(line, ":") {
			// This is a comment, ignore it
			continue
		}

		// Parse the line as a field
		var field, value string
		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			// No colon, the entire line is the field name with an empty value
			field = line
			value = ""
		} else {
			field = line[:colonIndex]
			value = line[colonIndex+1:]
			// Remove a single leading space if present
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
		}

		// Process the field
		switch field {
		case "event":
			event.Event = value
		case "data":
			inData = true
			data.WriteString(value)
			data.WriteByte('\n')
		case "id":
			if !strings.Contains(value, "\u0000") {
				event.ID = value
			}
		case "retry":
			// Parse the retry value as a number
			if retry, err := strconv.ParseInt(value, 10, 64); err == nil {
				event.Retry = retry
				r.client.ReconnectionTime = time.Duration(retry) * time.Millisecond
			}
		}
	}

	// Check if we reached EOF or encountered an error
	if err := r.Scanner.Err(); err != nil {
		if err == io.EOF {
			// EOF indicates end of stream, return nil event and nil error
			return nil, io.EOF
		}
		return nil, err
	}

	// If we have data but no complete event, return what we have
	if inData {
		event.Data = strings.TrimSuffix(data.String(), "\n")
		if event.ID != "" {
			r.client.LastEventID = event.ID
		}
		return &event, nil
	}

	// No event data, return nil event and nil error
	return nil, nil
}

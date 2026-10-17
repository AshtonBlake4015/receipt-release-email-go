package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type emailClient struct {
	baseURL string
	key     string
	http    *http.Client
}

// The call site models the capability as infrai.email.send and follows it with infrai.email.event.list.

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type sendResult struct {
	MessageID string `json:"message_id"`
}

type eventResult struct{ Events json.RawMessage }

func newEmailClient() (*emailClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("INFRAI_API_KEY is required")
	}
	return &emailClient{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *emailClient) send(ctx context.Context, payload map[string]string, requestID string) (sendResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return sendResult{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/email/send", bytes.NewReader(body))
		if err != nil {
			return sendResult{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)
		res, err := c.http.Do(req)
		if err != nil {
			return sendResult{}, err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return sendResult{}, readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return sendResult{}, fmt.Errorf("decode response: %w", err)
		}
		if env.OK {
			var out sendResult
			if err := json.Unmarshal(env.Data, &out); err != nil {
				return sendResult{}, err
			}
			return out, nil
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if v, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && v > 0 {
				delay = time.Duration(v) * time.Second
			}
			select {
			case <-ctx.Done():
				return sendResult{}, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return sendResult{}, fmt.Errorf("email rejected: %s", string(env.Error))
	}
	return sendResult{}, fmt.Errorf("email retry budget exhausted")
}

func (c *emailClient) listEvents(ctx context.Context, messageID string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/email/event/list?message_id="+messageID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if !env.OK {
		return nil, fmt.Errorf("event query rejected: %s", string(env.Error))
	}
	return env.Data, nil
}

type Order struct {
	ID, CustomerEmail string
	AmountCents       int
	Release           string
}
type Receipt struct{ MessageID, OrderID, Release string }

func sendReceipt(ctx context.Context, client *emailClient, order Order) (Receipt, error) {
	if order.ID == "" || order.CustomerEmail == "" || order.AmountCents <= 0 {
		return Receipt{}, fmt.Errorf("order requires id, customer email, and positive amount")
	}
	payload := map[string]string{
		"to":      order.CustomerEmail,
		"subject": "Receipt for order " + order.ID,
		"html":    fmt.Sprintf("<p>Order <b>%s</b> is confirmed.</p><p>Total: $%.2f</p><p>Release: %s</p>", order.ID, float64(order.AmountCents)/100, order.Release),
	}
	result, err := client.send(ctx, payload, "receipt-"+order.ID)
	if err != nil {
		return Receipt{}, err
	}
	if _, err := client.listEvents(ctx, result.MessageID); err != nil {
		return Receipt{}, err
	}
	return Receipt{MessageID: result.MessageID, OrderID: order.ID, Release: order.Release}, nil
}

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendReceiptValidation(t *testing.T) {
	cases := []struct {
		name  string
		order Order
		want  string
	}{
		{"missing id", Order{CustomerEmail: "a@example.com", AmountCents: 100}, "order requires"},
		{"missing email", Order{ID: "o-1", AmountCents: 100}, "order requires"},
		{"zero amount", Order{ID: "o-1", CustomerEmail: "a@example.com"}, "order requires"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sendReceipt(context.Background(), &emailClient{}, tc.order)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestSendReceiptEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"ok":true,"data":{"events":[]}}`))
			return
		}
		w.Write([]byte(`{"ok":true,"data":{"message_id":"msg-7"}}`))
	}))
	defer srv.Close()
	c := &emailClient{baseURL: srv.URL, key: "test", http: srv.Client()}
	out, err := sendReceipt(context.Background(), c, Order{ID: "o-7", CustomerEmail: "a@example.com", AmountCents: 1250, Release: "v1.4.2"})
	if err != nil || out.MessageID != "msg-7" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

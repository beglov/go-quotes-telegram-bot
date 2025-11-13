package quotes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientGetQuoteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("lang"); got != "ru" {
			t.Fatalf("expected lang=ru, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteText":"Жизнь прекрасна","quoteAuthor":"Альберт Эйнштейн"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), Config{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q, err := client.GetQuote(context.Background())
	if err != nil {
		t.Fatalf("GetQuote returned error: %v", err)
	}

	if q.Text != "Жизнь прекрасна" {
		t.Errorf("unexpected quote text: %q", q.Text)
	}

	if q.Author != "Альберт Эйнштейн" {
		t.Errorf("unexpected quote author: %q", q.Author)
	}
}

func TestClientGetQuoteMissingAuthor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteText":"Сложности делают нас сильнее","quoteAuthor":""}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), Config{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q, err := client.GetQuote(context.Background())
	if err != nil {
		t.Fatalf("GetQuote returned error: %v", err)
	}

	if q.Author != fallbackAuthor {
		t.Errorf("expected fallback author %q, got %q", fallbackAuthor, q.Author)
	}
}

func TestClientRetriesOnError(t *testing.T) {
	var calls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteText":"Ошибка превращается в успех","quoteAuthor":"Неизвестный"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), Config{
		BaseURL:    server.URL,
		RetryCount: 2,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := time.Now()
	q, err := client.GetQuote(context.Background())
	if err != nil {
		t.Fatalf("GetQuote returned error: %v", err)
	}

	if q.Text == "" {
		t.Error("expected non-empty quote text")
	}

	if atomic.LoadInt32(&calls) < 2 {
		t.Errorf("expected at least 2 attempts, got %d", calls)
	}

	if time.Since(start) < 10*time.Millisecond {
		t.Error("expected delay between attempts")
	}
}

func TestClientStopsAfterMaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), Config{
		BaseURL:    server.URL,
		RetryCount: 1,
		RetryDelay: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.GetQuote(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

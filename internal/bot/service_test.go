package bot

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/beglov/go-quotes-telegram-bot/internal/quotes"
)

type fakeBot struct {
	sentMessages []tgbotapi.MessageConfig
}

func (f *fakeBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func (f *fakeBot) StopReceivingUpdates() {}

func (f *fakeBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	msg, ok := c.(tgbotapi.MessageConfig)
	if !ok {
		panic("unexpected chattable type")
	}
	f.sentMessages = append(f.sentMessages, msg)
	return tgbotapi.Message{}, nil
}

type memoryStore struct {
	ids map[int64]struct{}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		ids: make(map[int64]struct{}),
	}
}

func (m *memoryStore) Subscribe(ctx context.Context, chatID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.ids[chatID] = struct{}{}
	return nil
}

func (m *memoryStore) Unsubscribe(ctx context.Context, chatID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	delete(m.ids, chatID)
	return nil
}

func (m *memoryStore) List(ctx context.Context) ([]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res := make([]int64, 0, len(m.ids))
	for id := range m.ids {
		res = append(res, id)
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	return res, nil
}

type stubQuotes struct {
	quote quotes.Quote
	err   error
}

func (s *stubQuotes) GetQuote(ctx context.Context) (quotes.Quote, error) {
	if err := ctx.Err(); err != nil {
		return quotes.Quote{}, err
	}
	if s.err != nil {
		return quotes.Quote{}, s.err
	}
	return s.quote, nil
}

type stubScheduler struct {
	lastCallback func(context.Context)
	started      bool
}

func (s *stubScheduler) Start(ctx context.Context, callback func(context.Context)) {
	s.started = true
	s.lastCallback = callback
}

func TestHandleUpdateStart(t *testing.T) {
	bot := &fakeBot{}
	store := newMemoryStore()
	quotes := &stubQuotes{quote: quotes.Quote{Text: "Тест", Author: "Автор"}}
	scheduler := &stubScheduler{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service, err := NewService(bot, quotes, store, scheduler, logger)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	update := commandUpdate(123, "/start")
	service.HandleUpdate(context.Background(), update)

	if _, exists := store.ids[123]; !exists {
		t.Fatal("expected chat 123 to be subscribed")
	}

	if len(bot.sentMessages) == 0 {
		t.Fatal("expected confirmation message to be sent")
	}

	got := bot.sentMessages[0].Text
	if got == "" || !strings.HasPrefix(got, "Вы подписались") {
		t.Fatalf("unexpected message text: %q", got)
	}
}

func TestHandleUpdateQuote(t *testing.T) {
	bot := &fakeBot{}
	store := newMemoryStore()
	quotes := &stubQuotes{quote: quotes.Quote{Text: "Будь собой", Author: "Сократ"}}
	scheduler := &stubScheduler{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service, err := NewService(bot, quotes, store, scheduler, logger)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	update := commandUpdate(42, "/quote")
	service.HandleUpdate(context.Background(), update)

	if len(bot.sentMessages) != 1 {
		t.Fatalf("expected one message, got %d", len(bot.sentMessages))
	}

	expected := "Будь собой\n\n— Сократ"
	if bot.sentMessages[0].Text != expected {
		t.Fatalf("unexpected quote message: %q", bot.sentMessages[0].Text)
	}
}

func TestSendDailyQuotes(t *testing.T) {
	bot := &fakeBot{}
	store := newMemoryStore()
	store.ids[1] = struct{}{}
	store.ids[2] = struct{}{}

	quotes := &stubQuotes{quote: quotes.Quote{Text: "Каждый день — новая возможность", Author: "Неизвестный"}}
	scheduler := &stubScheduler{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service, err := NewService(bot, quotes, store, scheduler, logger)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	service.sendDailyQuotes(context.Background())

	if len(bot.sentMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(bot.sentMessages))
	}

	expected := "Каждый день — новая возможность\n\n— Неизвестный"
	for _, msg := range bot.sentMessages {
		if msg.Text != expected {
			t.Fatalf("unexpected message: %q", msg.Text)
		}
	}
}

func commandUpdate(chatID int64, command string) tgbotapi.Update {
	length := len(command)
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: command,
			Entities: []tgbotapi.MessageEntity{
				{
					Type:   "bot_command",
					Offset: 0,
					Length: length,
				},
			},
		},
	}
}

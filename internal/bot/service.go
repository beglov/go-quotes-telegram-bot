package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/beglov/go-quotes-telegram-bot/internal/quotes"
)

// QuoteProvider описывает источник цитат.
type QuoteProvider interface {
	GetQuote(ctx context.Context) (quotes.Quote, error)
}

// SubscriberStore описывает хранилище подписчиков.
type SubscriberStore interface {
	Subscribe(ctx context.Context, chatID int64) error
	Unsubscribe(ctx context.Context, chatID int64) error
	List(ctx context.Context) ([]int64, error)
}

// Scheduler запускает периодическую рассылку.
type Scheduler interface {
	Start(ctx context.Context, callback func(context.Context))
}

// TelegramBot абстрагирует взаимодействие с Telegram Bot API.
type TelegramBot interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Service инкапсулирует бизнес-логику Telegram-бота.
type Service struct {
	bot          TelegramBot
	quotes       QuoteProvider
	subscribers  SubscriberStore
	scheduler    Scheduler
	logger       *slog.Logger
	updateConfig tgbotapi.UpdateConfig
}

// NewService конструирует сервис бота с необходимыми зависимостями.
func NewService(
	bot TelegramBot,
	quotes QuoteProvider,
	subscribers SubscriberStore,
	scheduler Scheduler,
	logger *slog.Logger,
) (*Service, error) {
	if bot == nil {
		return nil, errors.New("telegram bot is required")
	}
	if quotes == nil {
		return nil, errors.New("quote provider is required")
	}
	if subscribers == nil {
		return nil, errors.New("subscriber store is required")
	}
	if scheduler == nil {
		return nil, errors.New("scheduler is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 60

	return &Service{
		bot:          bot,
		quotes:       quotes,
		subscribers:  subscribers,
		scheduler:    scheduler,
		logger:       logger,
		updateConfig: cfg,
	}, nil
}

// Run запускает цикл обработки обновлений Telegram и ежедневную рассылку.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer s.bot.StopReceivingUpdates()

	s.scheduler.Start(ctx, s.sendDailyQuotes)
	updates := s.bot.GetUpdatesChan(s.updateConfig)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			s.HandleUpdate(ctx, update)
		}
	}
}

// HandleUpdate обрабатывает одиночное обновление Telegram.
func (s *Service) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}
	if !update.Message.IsCommand() {
		return
	}

	chatID := update.Message.Chat.ID
	command := update.Message.Command()

	switch command {
	case "start":
		if err := s.subscribers.Subscribe(ctx, chatID); err != nil {
			s.logger.Error("subscribe failed", "chat_id", chatID, "error", err)
			s.sendText(chatID, "Не удалось оформить подписку, попробуйте позже.")
			return
		}
		s.sendText(chatID, "Вы подписались на ежедневные цитаты. Приятного чтения!")
	case "stop":
		if err := s.subscribers.Unsubscribe(ctx, chatID); err != nil {
			s.logger.Error("unsubscribe failed", "chat_id", chatID, "error", err)
			s.sendText(chatID, "Не удалось отменить подписку, попробуйте позже.")
			return
		}
		s.sendText(chatID, "Вы отписались от рассылки. Возвращайтесь, когда соскучитесь по вдохновению!")
	case "quote":
		if err := s.sendQuote(ctx, chatID); err != nil {
			s.logger.Error("send quote failed", "chat_id", chatID, "error", err)
			s.sendText(chatID, "Не удалось получить цитату. Попробуйте еще раз.")
		}
	default:
		s.sendText(chatID, "Неизвестная команда. Доступные команды: /start, /stop, /quote.")
	}
}

func (s *Service) sendDailyQuotes(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	ids, err := s.subscribers.List(ctx)
	if err != nil {
		s.logger.Error("list subscribers failed", "error", err)
		return
	}

	if len(ids) == 0 {
		return
	}

	quote, err := s.quotes.GetQuote(ctx)
	if err != nil {
		s.logger.Error("get quote failed", "error", err)
		return
	}

	for _, chatID := range ids {
		if err := s.sendQuoteWithContent(ctx, chatID, quote); err != nil {
			s.logger.Error("send daily quote failed", "chat_id", chatID, "error", err)
		}
	}
}

func (s *Service) sendQuote(ctx context.Context, chatID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	quote, err := s.quotes.GetQuote(ctx)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}

	return s.sendQuoteWithContent(ctx, chatID, quote)
}

func (s *Service) sendQuoteWithContent(ctx context.Context, chatID int64, quote quotes.Quote) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	message := fmt.Sprintf("%s\n\n— %s", strings.TrimSpace(quote.Text), strings.TrimSpace(quote.Author))
	return s.sendText(chatID, message)
}

func (s *Service) sendText(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := s.bot.Send(msg); err != nil {
		return err
	}
	return nil
}

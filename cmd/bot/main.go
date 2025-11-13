package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/beglov/go-quotes-telegram-bot/internal/bot"
	"github.com/beglov/go-quotes-telegram-bot/internal/quotes"
	"github.com/beglov/go-quotes-telegram-bot/internal/scheduler"
	"github.com/beglov/go-quotes-telegram-bot/internal/subscribers"
)

type config struct {
	Token           string
	DailyHour       int
	DailyMinute     int
	SubscribersPath string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	quoteClient, err := quotes.NewClient(nil, quotes.Config{
		RetryCount: 3,
	})
	if err != nil {
		logger.Error("init quote client", "error", err)
		os.Exit(1)
	}

	store, err := subscribers.NewStore(context.Background(), cfg.SubscribersPath)
	if err != nil {
		logger.Error("init subscriber store", "error", err)
		os.Exit(1)
	}

	dailyScheduler, err := scheduler.NewDaily(cfg.DailyHour, cfg.DailyMinute, time.Local)
	if err != nil {
		logger.Error("init scheduler", "error", err)
		os.Exit(1)
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		logger.Error("init telegram bot", "error", err)
		os.Exit(1)
	}

	service, err := bot.NewService(botAPI, quoteClient, store, dailyScheduler, logger)
	if err != nil {
		logger.Error("init bot service", "error", err)
		os.Exit(1)
	}

	logger.Info("starting telegram bot", "daily_hour", cfg.DailyHour, "daily_minute", cfg.DailyMinute)

	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.Error("bot stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("bot stopped gracefully")
}

func loadConfig() (config, error) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}

	timeStr := strings.TrimSpace(os.Getenv("DAILY_TIME"))
	hour, minute := 8, 0
	if timeStr != "" {
		parts := strings.Split(timeStr, ":")
		if len(parts) != 2 {
			return config{}, fmt.Errorf("invalid DAILY_TIME format: %s", timeStr)
		}

		parsedHour, err := strconv.Atoi(parts[0])
		if err != nil {
			return config{}, fmt.Errorf("invalid DAILY_TIME hour: %w", err)
		}

		parsedMinute, err := strconv.Atoi(parts[1])
		if err != nil {
			return config{}, fmt.Errorf("invalid DAILY_TIME minute: %w", err)
		}

		hour = parsedHour
		minute = parsedMinute
	}

	subscribersPath := strings.TrimSpace(os.Getenv("SUBSCRIBERS_FILE"))
	if subscribersPath == "" {
		subscribersPath = "data/subscribers.json"
	}

	return config{
		Token:           token,
		DailyHour:       hour,
		DailyMinute:     minute,
		SubscribersPath: subscribersPath,
	}, nil
}

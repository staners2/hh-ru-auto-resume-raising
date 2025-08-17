package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"hh-ru-auto-resume-raising/internal/bot"
	"hh-ru-auto-resume-raising/internal/hh"
	"hh-ru-auto-resume-raising/internal/scheduler"
	"hh-ru-auto-resume-raising/internal/storage"
	"hh-ru-auto-resume-raising/pkg/config"
)

func main() {
	log.Println("Starting HH.ru auto resume raising bot...")

	// Загружаем конфигурацию
	cfg := config.Load()

	// Инициализируем хранилище
	store := storage.New()
	if err := store.Init(); err != nil {
		log.Fatal("Failed to initialize storage:", err)
	}

	// Создаем HH клиент
	hhClient, err := hh.NewClient(cfg.HHLogin, cfg.HHPassword, cfg.Proxy)
	if err != nil {
		log.Fatal("Failed to create HH client:", err)
	}

	// Загружаем токены
	if tokens, err := store.LoadTokens(); err == nil && tokens.XSRF != "" && tokens.HHToken != "" {
		hhClient.SetTokens(tokens.XSRF, tokens.HHToken)
		log.Println("Loaded existing tokens")
	} else {
		log.Println("No existing tokens found")
	}

	// Создаем планировщик
	sched := scheduler.New(hhClient, cfg.Timezone)

	// Загружаем расписание
	if schedules, err := store.LoadSchedule(); err == nil {
		for title, schedule := range schedules {
			sched.AddResume(title, schedule.ResumeID, schedule.Hour, schedule.Minute)
		}
		log.Printf("Loaded %d resume schedules", len(schedules))
	}

	// Создаем бота
	telegramBot, err := bot.New(cfg, hhClient, sched)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	// Устанавливаем обработчик уведомлений
	sched.SetNotificationHandler(telegramBot.SendNotification)

	// Запускаем планировщик
	sched.Start()
	defer sched.Stop()

	// Сохраняем токены при их обновлении (устанавливаем хук)
	go func() {
		for {
			// Можно добавить периодическое сохранение состояния
			// или реагировать на изменения
		}
	}()

	// Запускаем бота в отдельной горутине
	go func() {
		if err := telegramBot.Start(); err != nil {
			log.Fatal("Bot error:", err)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("Bot is running. Press Ctrl+C to exit.")
	<-c
	log.Println("Shutting down...")

	// Сохраняем текущее состояние перед выходом
	if xsrf, hhtoken := hhClient.GetTokens(); xsrf != "" && hhtoken != "" {
		if err := store.SaveTokens(xsrf, hhtoken); err != nil {
			log.Printf("Failed to save tokens: %v", err)
		}
	}

	if err := store.SaveSchedule(sched.GetAll()); err != nil {
		log.Printf("Failed to save schedule: %v", err)
	}

	log.Println("Bot stopped")
}
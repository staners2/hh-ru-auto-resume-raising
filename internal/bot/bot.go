package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"hh-ru-auto-resume-raising/internal/hh"
	"hh-ru-auto-resume-raising/internal/scheduler"
	"hh-ru-auto-resume-raising/pkg/config"
)

type UserState struct {
	State string
	Data  map[string]string
}

type Bot struct {
	api        *tgbotapi.BotAPI
	config     *config.Config
	hhClient   *hh.Client
	scheduler  *scheduler.Scheduler
	userStates map[int64]*UserState
}

func New(cfg *config.Config, hhClient *hh.Client, sched *scheduler.Scheduler) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		api:        api,
		config:     cfg,
		hhClient:   hhClient,
		scheduler:  sched,
		userStates: make(map[int64]*UserState),
	}, nil
}

func (b *Bot) Start() error {
	log.Printf("Authorized on account %s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			b.handleCallbackQuery(update.CallbackQuery)
		}
	}

	return nil
}

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	if message.From.ID != b.config.AdminTG {
		return
	}

	userID := message.Chat.ID

	// Проверяем, есть ли активное состояние у пользователя
	if state, exists := b.userStates[userID]; exists {
		b.handleState(message, state)
		return
	}

	// Обычная обработка команд
	switch message.Text {
	case "/start":
		b.sendMainMenu(message.Chat.ID)
	case "⚙ Профиль":
		b.handleProfile(message.Chat.ID)
	case "🔔 Вкл/выкл уведомления":
		b.handleToggleNotifications(message.Chat.ID)
	case "📜 Список резюме":
		b.handleListResumes(message.Chat.ID)
	case "📅 Расписание":
		b.handleShowSchedule(message.Chat.ID)
	case "➕ Добавить/обновить":
		b.handleAddResume(message.Chat.ID)
	case "❌ Удалить":
		b.handleDeleteResume(message.Chat.ID)
	case "🚀️ Авторизоваться":
		b.handleAuth(message.Chat.ID)
	case "📝 Обновить список резюме":
		b.handleUpdateResumes(message.Chat.ID)
	default:
		b.sendMainMenu(message.Chat.ID)
	}
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	if callback.From.ID != b.config.AdminTG {
		return
	}

	switch callback.Data {
	case "auth":
		b.handleAuth(callback.Message.Chat.ID)
	case "update_resumes":
		b.handleUpdateResumes(callback.Message.Chat.ID)
	case "schedule":
		b.handleShowSchedule(callback.Message.Chat.ID)
	case "toggle_notifications":
		b.handleToggleNotifications(callback.Message.Chat.ID)
	}

	b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func (b *Bot) sendMainMenu(chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⚙ Профиль"),
			tgbotapi.NewKeyboardButton("🔔 Вкл/выкл уведомления"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📜 Список резюме"),
			tgbotapi.NewKeyboardButton("📅 Расписание"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Добавить/обновить"),
			tgbotapi.NewKeyboardButton("❌ Удалить"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🚀️ Авторизоваться"),
			tgbotapi.NewKeyboardButton("📝 Обновить список резюме"),
		),
	)
	keyboard.ResizeKeyboard = true

	text := "HeadHunter Resume\nСервис автоматического подъема резюме каждые 4 часа."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleAuth(chatID int64) {
	err := b.hhClient.Login()
	var text string
	if err == nil {
		text = "✅ Авторизация успешна"
	} else {
		text = "❌ Ошибка авторизации: " + err.Error()
	}

	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

func (b *Bot) handleProfile(chatID int64) {
	text := fmt.Sprintf("<b>Ваши данные</b>\n"+
		"Логин: %s\n"+
		"Пароль: %s\n"+
		"Прокси: %s\n"+
		"Уведомления: %s",
		b.config.HHLogin,
		"***",
		func() string {
			if b.config.Proxy == "None" || b.config.Proxy == "" {
				return "не используется"
			}
			return b.config.Proxy
		}(),
		func() string {
			if b.scheduler.GetNotificationsEnabled() {
				return "включены"
			}
			return "отключены"
		}())

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleListResumes(chatID int64) {
	resumes, err := b.hhClient.GetResumes()
	if err != nil || len(resumes) == 0 {
		text := "<b>Резюме не найдено</b>\n" +
			"1) Попробуйте обновить список резюме.\n" +
			"2) Проверьте наличие резюме в профиле hh.ru"
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	text := "<b>Ваши резюме</b>"
	for _, resume := range resumes {
		text += fmt.Sprintf("\n\n<code>%s</code>", resume.Title)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleUpdateResumes(chatID int64) {
	resumes, err := b.hhClient.GetResumes()
	if err != nil {
		text := "Необходимо авторизоваться."
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	if len(resumes) > 0 {
		text := "<b>Ваши резюме</b>"
		for _, resume := range resumes {
			text += fmt.Sprintf("\n\n<code>%s</code>", resume.Title)
		}
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
	} else {
		text := "Резюме не найдены"
		msg := tgbotapi.NewMessage(chatID, text)
		b.api.Send(msg)
	}
}

func (b *Bot) handleAddResume(chatID int64) {
	// Получаем список резюме
	resumes, err := b.hhClient.GetResumes()
	if err != nil || len(resumes) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Обновите список резюме.")
		b.api.Send(msg)
		return
	}

	// Устанавливаем состояние ожидания названия резюме
	b.userStates[chatID] = &UserState{
		State: "add_resume_title",
		Data:  make(map[string]string),
	}

	msg := tgbotapi.NewMessage(chatID, "Введите наименование резюме, которое нужно поднимать.")
	b.api.Send(msg)
}

func (b *Bot) handleDeleteResume(chatID int64) {
	// Проверяем, есть ли резюме в расписании
	schedules := b.scheduler.GetAll()
	if len(schedules) == 0 {
		msg := tgbotapi.NewMessage(chatID, "В расписании нет резюме")
		b.api.Send(msg)
		return
	}

	// Устанавливаем состояние ожидания названия резюме для удаления
	b.userStates[chatID] = &UserState{
		State: "delete_resume_title",
		Data:  make(map[string]string),
	}

	msg := tgbotapi.NewMessage(chatID, "Введите наименование резюме, которое хотите удалить.")
	b.api.Send(msg)
}

func (b *Bot) handleState(message *tgbotapi.Message, state *UserState) {
	userID := message.Chat.ID

	switch state.State {
	case "add_resume_title":
		b.handleAddResumeTitle(message, state)
	case "add_resume_time":
		b.handleAddResumeTime(message, state)
	case "delete_resume_title":
		b.handleDeleteResumeTitle(message, state)
	default:
		// Неизвестное состояние, сбрасываем
		delete(b.userStates, userID)
		b.sendMainMenu(userID)
	}
}

func (b *Bot) handleAddResumeTitle(message *tgbotapi.Message, state *UserState) {
	userID := message.Chat.ID
	resumeTitle := message.Text

	// Проверяем, существует ли такое резюме
	resumes, err := b.hhClient.GetResumes()
	if err != nil {
		delete(b.userStates, userID)
		msg := tgbotapi.NewMessage(userID, "Ошибка получения списка резюме")
		b.api.Send(msg)
		return
	}

	found := false
	var resumeID string
	for _, resume := range resumes {
		if resume.Title == resumeTitle {
			found = true
			resumeID = resume.ID
			break
		}
	}

	if !found {
		delete(b.userStates, userID)
		msg := tgbotapi.NewMessage(userID, "Резюме с таким наименованием не найдено.")
		b.api.Send(msg)
		return
	}

	// Сохраняем данные и переходим к следующему состоянию
	state.Data["title"] = resumeTitle
	state.Data["resumeID"] = resumeID
	state.State = "add_resume_time"

	text := "Введите время поднятия, например 14:00 будет соответствовать " +
		"2:00 6:00 10:00 <code>14:00</code> 18:00 22:00"
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleAddResumeTime(message *tgbotapi.Message, state *UserState) {
	userID := message.Chat.ID
	timeStr := message.Text

	// Парсим время
	if !strings.Contains(timeStr, ":") {
		msg := tgbotapi.NewMessage(userID, "Ошибка при вводе времени, используйте формат 10:30.")
		b.api.Send(msg)
		return
	}

	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		msg := tgbotapi.NewMessage(userID, "Ошибка при вводе времени, используйте формат 10:30.")
		b.api.Send(msg)
		return
	}

	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		msg := tgbotapi.NewMessage(userID, "Ошибка при вводе времени, используйте формат 10:30.")
		b.api.Send(msg)
		return
	}

	// Добавляем резюме в расписание
	title := state.Data["title"]
	resumeID := state.Data["resumeID"]
	
	b.scheduler.AddResume(title, resumeID, hour, minute)
	
	text := fmt.Sprintf("<b>Добавлено новое расписание</b>\n%s\n%s", title, timeStr)
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)

	// Завершаем состояние
	delete(b.userStates, userID)
}

func (b *Bot) handleDeleteResumeTitle(message *tgbotapi.Message, state *UserState) {
	userID := message.Chat.ID
	resumeTitle := message.Text

	// Удаляем резюме из расписания
	removed := b.scheduler.RemoveResume(resumeTitle)
	
	var text string
	if removed {
		text = fmt.Sprintf("<b>Удалено следующее резюме</b>\n%s", resumeTitle)
	} else {
		text = "Резюме с таким наименованием не найдено."
	}

	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)

	// Завершаем состояние
	delete(b.userStates, userID)
}

func (b *Bot) handleShowSchedule(chatID int64) {
	schedules := b.scheduler.GetAll()
	if len(schedules) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📅 Расписание пусто")
		b.api.Send(msg)
		return
	}

	text := "📅 Текущее расписание:\n\n"
	for title, schedule := range schedules {
		text += fmt.Sprintf("📄 %s\n", title)
		text += fmt.Sprintf("⏰ %02d:%02d\n", schedule.Hour, schedule.Minute)
		text += fmt.Sprintf("🕐 Следующий запуск: %s\n\n",
			schedule.NextRun.Format("02.01.2006 15:04"))
	}

	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

func (b *Bot) handleToggleNotifications(chatID int64) {
	enabled := b.scheduler.ToggleNotifications()
	var text string
	if enabled {
		text = "🔔 Уведомления включены"
	} else {
		text = "🔕 Уведомления отключены"
	}

	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

func (b *Bot) SendNotification(message string) {
	if b.config.AdminTG != 0 {
		msg := tgbotapi.NewMessage(b.config.AdminTG, message)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
	}
}
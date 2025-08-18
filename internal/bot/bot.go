package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"hh-ru-auto-resume-raising/internal/hh"
	"hh-ru-auto-resume-raising/internal/scheduler"
	"hh-ru-auto-resume-raising/internal/storage"
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
	storage    *storage.Storage
	userStates map[int64]*UserState
}

func New(cfg *config.Config, hhClient *hh.Client, sched *scheduler.Scheduler, store *storage.Storage) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		api:        api,
		config:     cfg,
		hhClient:   hhClient,
		scheduler:  sched,
		storage:    store,
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

// getAuthStatus возвращает текст кнопки авторизации в зависимости от текущего статуса
func (b *Bot) getAuthStatus() string {
	// Проверяем авторизацию через попытку получить резюме
	_, err := b.hhClient.GetResumes()
	if err != nil {
		return "🔐 Войти в HeadHunter"
	}
	return "✅ Авторизован"
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
	case "⚙ Настройки":
		b.handleSettingsMenu(message.Chat.ID)
	case "ℹ️ Помощь":
		b.handleHelp(message.Chat.ID)
	case "🔔 Уведомления":
		b.handleToggleNotifications(message.Chat.ID)
	case "👤 Профиль":
		b.handleProfile(message.Chat.ID)
	case "↩️ Главное меню":
		b.sendMainMenu(message.Chat.ID)
	case "📜 Мои резюме":
		b.handleListResumes(message.Chat.ID)
	case "📅 Расписание":
		b.handleShowSchedule(message.Chat.ID)
	case "➕ Настроить подъем":
		b.handleAddResumeWithMessage(message)
	case "❌ Удалить из расписания":
		b.handleDeleteResumeWithMessage(message)
	case "🔐 Войти в HeadHunter", "✅ Авторизован":
		b.handleAuth(message.Chat.ID)
	case "🔄 Обновить данные":
		b.handleUpdateResumes(message.Chat.ID)
	// Поддержка старых команд для обратной совместимости
	case "🔔 Вкл/выкл уведомления":
		b.handleToggleNotifications(message.Chat.ID)
	case "📜 Список резюме":
		b.handleListResumes(message.Chat.ID)
	case "➕ Добавить/обновить":
		b.handleAddResumeWithMessage(message)
	case "❌ Удалить":
		b.handleDeleteResumeWithMessage(message)
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

	switch {
	case callback.Data == "auth":
		b.handleAuth(callback.Message.Chat.ID)
	case callback.Data == "update_resumes":
		b.handleUpdateResumes(callback.Message.Chat.ID)
	case callback.Data == "schedule":
		b.handleShowSchedule(callback.Message.Chat.ID)
	case callback.Data == "toggle_notifications":
		b.handleToggleNotifications(callback.Message.Chat.ID)
	case callback.Data == "cancel_add_resume":
		b.handleCancelAddResume(callback)
	case callback.Data == "cancel_delete_resume":
		b.handleCancelDeleteResume(callback)
	case strings.HasPrefix(callback.Data, "add_resume:"):
		b.handleAddResumeCallback(callback)
	case strings.HasPrefix(callback.Data, "delete_resume:"):
		b.handleDeleteResumeCallback(callback)
	}

	b.api.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func (b *Bot) sendMainMenu(chatID int64) {
	// Проверяем статус авторизации для динамической адаптации кнопок
	authStatus := b.getAuthStatus()
	
	var keyboard tgbotapi.ReplyKeyboardMarkup
	
	// Если не авторизован - показываем упрощенное меню с фокусом на авторизацию
	if authStatus == "🔐 Войти в HeadHunter" {
		keyboard = tgbotapi.NewReplyKeyboard(
			// Ряд 1: Приоритет авторизации
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("🔐 Войти в HeadHunter"),
			),
			// Ряд 2: Базовая информация
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⚙ Настройки"),
				tgbotapi.NewKeyboardButton("ℹ️ Помощь"),
			),
		)
	} else {
		// Полное меню для авторизованных пользователей
		keyboard = tgbotapi.NewReplyKeyboard(
			// Ряд 1: Статус авторизации (успешно)
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("✅ Авторизован"),
			),
			// Ряд 2: Основные операции с резюме (наиболее частые действия)
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("📜 Мои резюме"),
				tgbotapi.NewKeyboardButton("📅 Расписание"),
			),
			// Ряд 3: Управление автоподъемом (основная функциональность)
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("➕ Настроить подъем"),
				tgbotapi.NewKeyboardButton("❌ Удалить из расписания"),
			),
			// Ряд 4: Системные функции (реже используемые)
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⚙ Настройки"),
				tgbotapi.NewKeyboardButton("🔄 Обновить данные"),
			),
		)
	}
	
	keyboard.ResizeKeyboard = true

	// Контекстное приветственное сообщение
	text := "🎯 <b>HeadHunter Auto Resume</b>\n\n"
	text += "Автоматический подъем резюме каждые 4 часа\n"
	
	// Добавляем контекстную информацию в зависимости от состояния
	if authStatus == "🔐 Войти в HeadHunter" {
		text += "\n🚀 <b>Добро пожаловать!</b>\n"
		text += "⚠️ <i>Для начала работы необходимо войти в ваш аккаунт HeadHunter</i>\n\n"
		text += "📋 <b>После авторизации вы сможете:</b>\n"
		text += "• Просматривать свои резюме\n"
		text += "• Настраивать автоматический подъем\n"
		text += "• Управлять расписанием обновлений"
	} else {
		schedules := b.scheduler.GetAll()
		text += "\n✅ <b>Система готова к работе</b>\n"
		
		if len(schedules) == 0 {
			text += "💡 <i>Рекомендуем настроить автоподъем для ваших резюме</i>\n"
			text += "📌 Нажмите \"➕ Настроить подъем\" для начала"
		} else {
			text += fmt.Sprintf("🔥 <i>Активно автоподъемов: %d</i>\n", len(schedules))
			text += "📈 Ваши резюме регулярно обновляются"
		}
		
		// Добавляем статус уведомлений
		if b.scheduler.GetNotificationsEnabled() {
			text += "\n🔔 Уведомления включены"
		} else {
			text += "\n🔕 Уведомления отключены"
		}
	}
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleAuth(chatID int64) {
	// Если уже авторизован, показываем статус
	if _, err := b.hhClient.GetResumes(); err == nil {
		text := "✅ <b>Вы уже авторизованы</b>\n\nПодключение к HeadHunter активно. Можете настраивать автоподъем резюме."
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	// Показываем процесс авторизации
	processingMsg := tgbotapi.NewMessage(chatID, "🔄 <b>Авторизация...</b>\n\nПодключаемся к HeadHunter...")
	processingMsg.ParseMode = "HTML"
	b.api.Send(processingMsg)

	err := b.hhClient.Login()
	var text string
	if err == nil {
		text = "✅ <b>Авторизация успешна!</b>\n\nТеперь вы можете:\n• Просматривать свои резюме\n• Настраивать автоподъем\n• Управлять расписанием"
		// Сохраняем токены после успешной авторизации
		if xsrf, hhtoken := b.hhClient.GetTokens(); xsrf != "" && hhtoken != "" {
			if saveErr := b.storage.SaveTokens(xsrf, hhtoken); saveErr != nil {
				log.Printf("Failed to save tokens: %v", saveErr)
			} else {
				log.Println("Tokens saved successfully")
			}
		}
		// Обновляем главное меню для показа нового статуса
		b.sendMainMenu(chatID)
		return
	} else {
		text = "❌ <b>Ошибка авторизации</b>\n\n" + err.Error() + "\n\n💡 Проверьте настройки логина и пароля в конфигурации."
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleProfile(chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("↩️ Главное меню"),
		),
	)
	keyboard.ResizeKeyboard = true

	// Проверяем статус авторизации для более детальной информации
	authStatus := "❌ Не авторизован"
	if _, err := b.hhClient.GetResumes(); err == nil {
		authStatus = "✅ Активна"
	}

	text := "👤 <b>Профиль пользователя</b>\n\n"
	text += fmt.Sprintf("🔐 Статус авторизации: <b>%s</b>\n", authStatus)
	text += fmt.Sprintf("👨‍💼 Логин HeadHunter: <code>%s</code>\n", b.config.HHLogin)
	text += "🔒 Пароль: <code>***</code>\n"
	
	proxyText := "не используется"
	if b.config.Proxy != "None" && b.config.Proxy != "" {
		proxyText = b.config.Proxy
	}
	text += fmt.Sprintf("🌐 Прокси: <code>%s</code>\n", proxyText)
	
	notificationsText := "отключены"
	if b.scheduler.GetNotificationsEnabled() {
		notificationsText = "включены"
	}
	text += fmt.Sprintf("🔔 Уведомления: <b>%s</b>\n", notificationsText)
	
	// Добавляем информацию о расписаниях
	schedules := b.scheduler.GetAll()
	text += fmt.Sprintf("📅 Активных расписаний: <b>%d</b>\n", len(schedules))
	
	text += "\n💡 <i>Настройки системы можно изменить через конфигурационный файл</i>"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleListResumes(chatID int64) {
	resumes, err := b.hhClient.GetResumes()
	if err != nil {
		text := "❌ <b>Не удалось загрузить резюме</b>\n\n"
		if _, authErr := b.hhClient.GetResumes(); authErr != nil {
			text += "Необходимо авторизоваться в HeadHunter.\n\n💡 Нажмите кнопку \"🔐 Войти в HeadHunter\""
		} else {
			text += err.Error() + "\n\n💡 Попробуйте \"🔄 Обновить данные\""
		}
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	if len(resumes) == 0 {
		text := "📝 <b>Резюме не найдены</b>\n\n" +
			"Создайте резюме на hh.ru и обновите данные в боте.\n\n" +
			"💡 Используйте кнопку \"🔄 Обновить данные\""
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	// Получаем информацию о расписании для каждого резюме
	schedules := b.scheduler.GetAll()
	
	text := fmt.Sprintf("📜 <b>Ваши резюме (%d)</b>\n\n", len(resumes))
	for i, resume := range resumes {
		text += fmt.Sprintf("%d. <code>%s</code>", i+1, resume.Title)
		
		// Проверяем, есть ли расписание для этого резюме
		if schedule, exists := schedules[resume.Title]; exists {
			text += fmt.Sprintf("\n   ⏰ Автоподъем: %02d:%02d", schedule.Hour, schedule.Minute)
			text += fmt.Sprintf("\n   🕐 Следующий: %s", schedule.NextRun.Format("02.01 15:04"))
		} else {
			text += "\n   ➕ Автоподъем не настроен"
		}
		
		if i < len(resumes)-1 {
			text += "\n\n"
		}
	}

	if len(schedules) == 0 {
		text += "\n\n💡 <i>Настройте автоподъем с помощью кнопки \"➕ Настроить подъем\"</i>"
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleUpdateResumes(chatID int64) {
	// Показываем процесс обновления
	processingMsg := tgbotapi.NewMessage(chatID, "🔄 <b>Обновляем данные...</b>\n\nЗагружаем актуальную информацию с HeadHunter...")
	processingMsg.ParseMode = "HTML"
	b.api.Send(processingMsg)

	resumes, err := b.hhClient.GetResumes()
	if err != nil {
		text := "❌ <b>Ошибка обновления данных</b>\n\n"
		text += "Необходимо авторизоваться.\n\n"
		text += "💡 Нажмите кнопку \"🔐 Войти в HeadHunter\""
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	if len(resumes) > 0 {
		schedules := b.scheduler.GetAll()
		
		text := fmt.Sprintf("✅ <b>Данные обновлены</b>\n\nНайдено резюме: %d\n", len(resumes))
		
		activeSchedules := 0
		for _, resume := range resumes {
			if _, exists := schedules[resume.Title]; exists {
				activeSchedules++
			}
		}
		
		if activeSchedules > 0 {
			text += fmt.Sprintf("Настроено автоподъемов: %d\n\n", activeSchedules)
			text += "📜 Используйте \"📜 Мои резюме\" для подробной информации"
		} else {
			text += "\n💡 <i>Настройте автоподъем с помощью кнопки \"➕ Настроить подъем\"</i>"
		}
		
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
	} else {
		text := "⚠️ <b>Резюме не найдены</b>\n\n"
		text += "Создайте резюме на hh.ru и повторите обновление.\n\n"
		text += "🔗 Перейдите на hh.ru → Мои резюме"
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
	}
}

func (b *Bot) handleAddResumeWithMessage(message *tgbotapi.Message) {
	b.handleAddResume(message.Chat.ID, message.MessageID)
}

func (b *Bot) handleDeleteResumeWithMessage(message *tgbotapi.Message) {
	b.handleDeleteResume(message.Chat.ID, message.MessageID)
}

func (b *Bot) handleAddResume(chatID int64, originalMessageID ...int) {
	// Получаем список резюме
	resumes, err := b.hhClient.GetResumes()
	if err != nil || len(resumes) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Обновите список резюме.")
		b.api.Send(msg)
		return
	}

	// Получаем текущие расписания для отображения времени
	schedules := b.scheduler.GetAll()

	// Создаем inline клавиатуру с резюме
	var keyboard [][]tgbotapi.InlineKeyboardButton
	
	for _, resume := range resumes {
		buttonText := resume.Title
		
		// Проверяем, есть ли уже расписание для этого резюме
		if schedule, exists := schedules[resume.Title]; exists {
			buttonText += fmt.Sprintf(" ⏰ %02d:%02d", schedule.Hour, schedule.Minute)
		} else {
			buttonText += " ➕"
		}
		
		button := tgbotapi.NewInlineKeyboardButtonData(
			buttonText,
			fmt.Sprintf("add_resume:%s", resume.ID),
		)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	// Добавляем кнопку отмены
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel_add_resume")
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{cancelButton})

	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	
	text := "<b>Выберите резюме для настройки автоподъема:</b>\n\n"
	text += "⏰ - уже настроено (время подъема)\n"
	text += "➕ - не настроено"
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	
	// Сохраняем ID отправленного сообщения для возможности удаления при отмене
	sentMsg, _ := b.api.Send(msg)
	if b.userStates == nil {
		b.userStates = make(map[int64]*UserState)
	}
	
	data := map[string]string{
		"resume_list_message_id": fmt.Sprintf("%d", sentMsg.MessageID),
	}
	
	// Сохраняем ID оригинального сообщения если оно передано
	if len(originalMessageID) > 0 {
		data["original_message_id"] = fmt.Sprintf("%d", originalMessageID[0])
	}
	
	b.userStates[chatID] = &UserState{
		State: "showing_resume_list",
		Data:  data,
	}
}

func (b *Bot) handleDeleteResume(chatID int64, originalMessageID ...int) {
	// Проверяем, есть ли резюме в расписании
	schedules := b.scheduler.GetAll()
	if len(schedules) == 0 {
		text := "📅 <b>Расписание пусто</b>\n\n"
		text += "Нет резюме для удаления из автоподъема.\n\n"
		text += "💡 Сначала настройте автоподъем с помощью кнопки \"➕ Настроить подъем\""
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	// Создаем inline клавиатуру с резюме в расписании
	var keyboard [][]tgbotapi.InlineKeyboardButton
	
	for title, schedule := range schedules {
		buttonText := fmt.Sprintf("❌ %s ⏰ %02d:%02d", title, schedule.Hour, schedule.Minute)
		
		button := tgbotapi.NewInlineKeyboardButtonData(
			buttonText,
			fmt.Sprintf("delete_resume:%s", title),
		)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	// Добавляем кнопку отмены
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("↩️ Отмена", "cancel_delete_resume")
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{cancelButton})

	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	
	text := "❌ <b>Удалить из расписания</b>\n\n"
	text += fmt.Sprintf("Активных автоподъемов: %d\n\n", len(schedules))
	text += "Выберите резюме для удаления из автоподъема:"
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	
	// Сохраняем ID отправленного сообщения для возможности удаления при отмене
	sentMsg, _ := b.api.Send(msg)
	if b.userStates == nil {
		b.userStates = make(map[int64]*UserState)
	}
	
	data := map[string]string{
		"delete_list_message_id": fmt.Sprintf("%d", sentMsg.MessageID),
	}
	
	// Сохраняем ID оригинального сообщения если оно передано
	if len(originalMessageID) > 0 {
		data["original_message_id"] = fmt.Sprintf("%d", originalMessageID[0])
	}
	
	b.userStates[chatID] = &UserState{
		State: "showing_delete_list",
		Data:  data,
	}
}

func (b *Bot) handleState(message *tgbotapi.Message, state *UserState) {
	userID := message.Chat.ID

	switch state.State {
	case "add_resume_time":
		b.handleAddResumeTime(message, state)
	default:
		// Неизвестное состояние, сбрасываем
		delete(b.userStates, userID)
		b.sendMainMenu(userID)
	}
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
	
	// Сохраняем расписание
	if err := b.storage.SaveSchedule(b.scheduler.GetAll()); err != nil {
		log.Printf("Failed to save schedule: %v", err)
	} else {
		log.Println("Schedule saved successfully")
	}
	
	// Рассчитываем следующие времена подъема
	nextTimes := []string{}
	baseTime := fmt.Sprintf("%02d:%02d", hour, minute)
	for i := 0; i < 4; i++ {
		nextHour := (hour + i*4) % 24
		nextTimes = append(nextTimes, fmt.Sprintf("%02d:%02d", nextHour, minute))
	}
	
	text := fmt.Sprintf("✅ <b>Автоподъем настроен!</b>\n\n")
	text += fmt.Sprintf("Резюме: <code>%s</code>\n", title)
	text += fmt.Sprintf("⏰ Первый подъем: <b>%s</b>\n\n", baseTime)
	text += "🔄 <b>Расписание на день:</b>\n"
	for _, time := range nextTimes {
		text += fmt.Sprintf("• %s\n", time)
	}
	text += "\n💡 <i>Автоподъем активен! Проверить статус можно в разделе \"📅 Расписание\"</i>"
	
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)

	// Завершаем состояние
	delete(b.userStates, userID)
}


func (b *Bot) handleShowSchedule(chatID int64) {
	schedules := b.scheduler.GetAll()
	if len(schedules) == 0 {
		text := "📅 <b>Расписание пусто</b>\n\n"
		text += "Автоподъем резюме не настроен.\n\n"
		text += "💡 Используйте кнопку \"➕ Настроить подъем\" для добавления резюме в автоподъем."
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
		return
	}

	notificationsStatus := "включены"
	if !b.scheduler.GetNotificationsEnabled() {
		notificationsStatus = "отключены"
	}

	text := fmt.Sprintf("📅 <b>Расписание автоподъема (%d)</b>\n\n", len(schedules))
	text += fmt.Sprintf("🔔 Уведомления: %s\n\n", notificationsStatus)
	
	i := 1
	for title, schedule := range schedules {
		text += fmt.Sprintf("<b>%d.</b> <code>%s</code>\n", i, title)
		text += fmt.Sprintf("   ⏰ Время: <b>%02d:%02d</b>\n", schedule.Hour, schedule.Minute)
		text += fmt.Sprintf("   🕐 Следующий запуск: <i>%s</i>\n", 
			schedule.NextRun.Format("02.01 15:04"))
		
		if !schedule.LastRun.IsZero() {
			text += fmt.Sprintf("   ✅ Последний: <i>%s</i>\n", 
				schedule.LastRun.Format("02.01 15:04"))
		}
		
		if i < len(schedules) {
			text += "\n"
		}
		i++
	}
	
	text += "\n💡 <i>Резюме поднимаются автоматически каждые 4 часа</i>"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleToggleNotifications(chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("↩️ Главное меню"),
		),
	)
	keyboard.ResizeKeyboard = true

	enabled := b.scheduler.ToggleNotifications()
	var text string
	if enabled {
		text = "🔔 <b>Уведомления включены</b>\n\n"
		text += "✅ Вы будете получать сообщения о результатах автоподъема резюме\n\n"
		text += "📬 <b>Вы будете уведомлены о:</b>\n"
		text += "• Успешном подъеме резюме\n"
		text += "• Ошибках при подъеме\n"
		text += "• Проблемах авторизации\n"
		text += "• Изменениях в расписании"
	} else {
		text = "🔕 <b>Уведомления отключены</b>\n\n"
		text += "❌ Автоподъем продолжит работать в фоновом режиме без уведомлений\n\n"
		text += "💡 <i>Включить уведомления можно в любой момент через настройки</i>"
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleCancelAddResume(callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	
	// Удаляем сообщение с кнопками
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	b.api.Request(deleteMsg)
	
	// Удаляем оригинальное сообщение "➕ Добавить/обновить" если есть
	if state, exists := b.userStates[chatID]; exists {
		if originalMsgID := state.Data["original_message_id"]; originalMsgID != "" {
			if msgID, err := strconv.Atoi(originalMsgID); err == nil {
				deleteOriginal := tgbotapi.NewDeleteMessage(chatID, msgID)
				b.api.Request(deleteOriginal)
			}
		}
	}
	
	// Очищаем состояние пользователя
	delete(b.userStates, chatID)
}

func (b *Bot) handleAddResumeCallback(callback *tgbotapi.CallbackQuery) {
	// Извлекаем ID резюме из callback data
	resumeID := strings.TrimPrefix(callback.Data, "add_resume:")
	
	// Найдем резюме по ID чтобы получить название
	resumes, err := b.hhClient.GetResumes()
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Ошибка получения списка резюме")
		b.api.Send(msg)
		return
	}
	
	var resumeTitle string
	for _, resume := range resumes {
		if resume.ID == resumeID {
			resumeTitle = resume.Title
			break
		}
	}
	
	if resumeTitle == "" {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Резюме не найдено")
		b.api.Send(msg)
		return
	}
	
	// Удаляем сообщение с кнопками
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	b.api.Request(deleteMsg)
	
	// Устанавливаем состояние ожидания времени
	b.userStates[callback.Message.Chat.ID] = &UserState{
		State: "add_resume_time",
		Data: map[string]string{
			"title":    resumeTitle,
			"resumeID": resumeID,
		},
	}
	
	text := fmt.Sprintf("⏰ <b>Настройка автоподъема</b>\n\n")
	text += fmt.Sprintf("Резюме: <code>%s</code>\n\n", resumeTitle)
	text += "📋 <b>Введите время первого подъема</b> (формат ЧЧ:ММ)\n"
	text += "Например: <code>09:00</code> или <code>14:30</code>\n\n"
	text += "🔄 Далее резюме будет подниматься <b>каждые 4 часа</b>\n"
	text += "💡 Рекомендуемое время: 09:00 (подъемы в 9:00, 13:00, 17:00, 21:00)"
	
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) handleSettingsMenu(chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		// Ряд 1: Настройки профиля и уведомлений
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("👤 Профиль"),
			tgbotapi.NewKeyboardButton("🔔 Уведомления"),
		),
		// Ряд 2: Системные функции
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔄 Обновить данные"),
			tgbotapi.NewKeyboardButton("ℹ️ Помощь"),
		),
		// Ряд 3: Возврат в главное меню
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("↩️ Главное меню"),
		),
	)
	keyboard.ResizeKeyboard = true

	// Получаем текущие настройки для отображения
	notificationsStatus := "включены"
	if !b.scheduler.GetNotificationsEnabled() {
		notificationsStatus = "отключены"
	}
	
	schedules := b.scheduler.GetAll()
	
	text := "⚙️ <b>Настройки системы</b>\n\n"
	text += fmt.Sprintf("🔔 Уведомления: <b>%s</b>\n", notificationsStatus)
	text += fmt.Sprintf("📋 Резюме в автоподъеме: <b>%d</b>\n", len(schedules))
	
	// Проверяем статус авторизации
	if _, err := b.hhClient.GetResumes(); err == nil {
		text += "🔐 Авторизация: <b>✅ Активна</b>\n"
	} else {
		text += "🔐 Авторизация: <b>❌ Требуется</b>\n"
	}
	
	text += "\n💡 <i>Выберите раздел для настройки</i>"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleHelp(chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("↩️ Главное меню"),
		),
	)
	keyboard.ResizeKeyboard = true

	text := "ℹ️ <b>Справка по использованию</b>\n\n"
	text += "🎯 <b>Основные функции:</b>\n"
	text += "• <b>Авторизация</b> - подключение к вашему аккаунту HeadHunter\n"
	text += "• <b>Мои резюме</b> - просмотр всех ваших резюме\n"
	text += "• <b>Настроить подъем</b> - автоматический подъем каждые 4 часа\n"
	text += "• <b>Расписание</b> - управление временем подъема резюме\n\n"
	
	text += "⏰ <b>Как работает автоподъем:</b>\n"
	text += "1. Выберите резюме для автоподъема\n"
	text += "2. Укажите время первого подъема (например, 09:00)\n"
	text += "3. Система будет поднимать резюме каждые 4 часа\n"
	text += "   Пример: 09:00 → 13:00 → 17:00 → 21:00\n\n"
	
	text += "🔔 <b>Уведомления:</b>\n"
	text += "Получайте сообщения о результатах автоподъема\n\n"
	
	text += "⚠️ <b>Важно:</b>\n"
	text += "• Резюме поднимается максимум раз в 4 часа (ограничение HH)\n"
	text += "• Для работы требуется активная авторизация\n"
	text += "• Бот работает автоматически 24/7"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleDeleteResumeCallback(callback *tgbotapi.CallbackQuery) {
	// Извлекаем название резюме из callback data
	resumeTitle := strings.TrimPrefix(callback.Data, "delete_resume:")
	
	// Удаляем сообщение с кнопками
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	b.api.Request(deleteMsg)
	
	// Удаляем резюме из расписания
	removed := b.scheduler.RemoveResume(resumeTitle)
	
	var text string
	if removed {
		text = "✅ <b>Удалено из автоподъема</b>\n\n"
		text += fmt.Sprintf("Резюме: <code>%s</code>\n\n", resumeTitle)
		text += "Автоподъем для этого резюме отключен.\n"
		text += "При необходимости можете настроить заново."
		
		// Сохраняем расписание после удаления
		if err := b.storage.SaveSchedule(b.scheduler.GetAll()); err != nil {
			log.Printf("Failed to save schedule: %v", err)
		} else {
			log.Println("Schedule saved successfully after deletion")
		}
	} else {
		text = "❌ <b>Ошибка удаления</b>\n\n"
		text += fmt.Sprintf("Резюме \"%s\" не найдено в расписании.", resumeTitle)
	}

	// Удаляем оригинальное сообщение "❌ Удалить из расписания" если есть
	chatID := callback.Message.Chat.ID
	if state, exists := b.userStates[chatID]; exists {
		if originalMsgID := state.Data["original_message_id"]; originalMsgID != "" {
			if msgID, err := strconv.Atoi(originalMsgID); err == nil {
				deleteOriginal := tgbotapi.NewDeleteMessage(chatID, msgID)
				b.api.Request(deleteOriginal)
			}
		}
	}
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
	
	// Очищаем состояние пользователя
	delete(b.userStates, chatID)
}

func (b *Bot) handleCancelDeleteResume(callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	
	// Удаляем сообщение с кнопками
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	b.api.Request(deleteMsg)
	
	// Удаляем оригинальное сообщение "❌ Удалить из расписания" если есть
	if state, exists := b.userStates[chatID]; exists {
		if originalMsgID := state.Data["original_message_id"]; originalMsgID != "" {
			if msgID, err := strconv.Atoi(originalMsgID); err == nil {
				deleteOriginal := tgbotapi.NewDeleteMessage(chatID, msgID)
				b.api.Request(deleteOriginal)
			}
		}
	}
	
	// Очищаем состояние пользователя
	delete(b.userStates, chatID)
}

func (b *Bot) SendNotification(message string) {
	if b.config.AdminTG != 0 {
		msg := tgbotapi.NewMessage(b.config.AdminTG, message)
		msg.ParseMode = "HTML"
		b.api.Send(msg)
	}
}
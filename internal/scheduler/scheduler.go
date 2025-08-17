package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"hh-ru-auto-resume-raising/internal/hh"
)

type ResumeSchedule struct {
	ResumeID  string    `json:"resume_id"`
	Hour      int       `json:"hour"`
	Minute    int       `json:"minute"`
	NextRun   time.Time `json:"next_run"`
	LastRun   time.Time `json:"last_run"`
}

type NotificationHandler func(message string)

type Scheduler struct {
	cron          *cron.Cron
	schedules     map[string]ResumeSchedule
	hhClient      *hh.Client
	notifications bool
	notifyHandler NotificationHandler
	mutex         sync.RWMutex
}

func New(hhClient *hh.Client, timezone string) *Scheduler {
	loc, _ := time.LoadLocation(timezone)
	return &Scheduler{
		cron:          cron.New(cron.WithLocation(loc)),
		schedules:     make(map[string]ResumeSchedule),
		hhClient:      hhClient,
		notifications: true,
	}
}

func (s *Scheduler) SetNotificationHandler(handler NotificationHandler) {
	s.notifyHandler = handler
}

func (s *Scheduler) Start() {
	// Добавляем задачу, которая выполняется каждую минуту
	s.cron.AddFunc("* * * * *", func() {
		s.checkAndRaiseResumes()
	})
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) AddResume(title, resumeID string, hour, minute int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if nextRun.Before(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	s.schedules[title] = ResumeSchedule{
		ResumeID: resumeID,
		Hour:     hour,
		Minute:   minute,
		NextRun:  nextRun,
		LastRun:  time.Time{},
	}
}

func (s *Scheduler) RemoveResume(title string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.schedules[title]
	if exists {
		delete(s.schedules, title)
	}
	return exists
}

func (s *Scheduler) GetAll() map[string]ResumeSchedule {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make(map[string]ResumeSchedule)
	for k, v := range s.schedules {
		result[k] = v
	}
	return result
}

func (s *Scheduler) ToggleNotifications() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.notifications = !s.notifications
	return s.notifications
}

func (s *Scheduler) GetNotificationsEnabled() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.notifications
}

func (s *Scheduler) checkAndRaiseResumes() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()

	for title, schedule := range s.schedules {
		if now.After(schedule.NextRun) && now.Sub(schedule.LastRun) >= 4*time.Hour {
			go s.raiseResumeAsync(title, schedule)
		}
	}
}

func (s *Scheduler) raiseResumeAsync(title string, schedule ResumeSchedule) {
	code, err := s.hhClient.RaiseResume(schedule.ResumeID)
	if err != nil {
		log.Printf("Error raising resume %s: %v", title, err)
		return
	}

	if code == 409 || code == 200 {
		// Успешно или уже поднято недавно
		s.updateScheduleNextRun(title)
	} else {
		// Попробуем переавторизоваться
		if err := s.hhClient.Login(); err == nil {
			code, err = s.hhClient.RaiseResume(schedule.ResumeID)
			if err == nil && (code == 409 || code == 200) {
				s.updateScheduleNextRun(title)
			}
		}
	}

	if s.notifications && s.notifyHandler != nil {
		statusText := s.getStatusText(code)
		text := fmt.Sprintf("📄 <b>%s</b>\n%s\n🕐 %s",
			title, statusText, time.Now().Format("15:04:05"))
		s.notifyHandler(text)
	}
}

func (s *Scheduler) updateScheduleNextRun(title string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if schedule, exists := s.schedules[title]; exists {
		schedule.LastRun = time.Now()
		schedule.NextRun = time.Now().Add(4 * time.Hour)
		s.schedules[title] = schedule
	}
}

func (s *Scheduler) getStatusText(code int) string {
	switch code {
	case 200:
		return "✅ Резюме успешно поднято"
	case 409:
		return "⏳ Резюме уже поднималось недавно"
	case 403:
		return "❌ Доступ запрещен"
	case 429:
		return "⏸️ Слишком много запросов"
	default:
		return fmt.Sprintf("❌ Ошибка: %d", code)
	}
}
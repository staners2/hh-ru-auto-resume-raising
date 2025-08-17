package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"hh-ru-auto-resume-raising/internal/scheduler"
)

const (
	configDir     = "config"
	tokensFile    = "tokens.json"
	scheduleFile  = "schedule.json"
)

type TokenData struct {
	XSRF    string `json:"xsrf"`
	HHToken string `json:"hhtoken"`
}

type Storage struct {
	configPath string
}

func New() *Storage {
	return &Storage{
		configPath: configDir,
	}
}

func (s *Storage) Init() error {
	return os.MkdirAll(s.configPath, 0755)
}

func (s *Storage) LoadTokens() (*TokenData, error) {
	tokensPath := filepath.Join(s.configPath, tokensFile)
	
	if _, err := os.Stat(tokensPath); os.IsNotExist(err) {
		return &TokenData{}, nil
	}

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, err
	}

	var tokens TokenData
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}

	return &tokens, nil
}

func (s *Storage) SaveTokens(xsrf, hhtoken string) error {
	if err := s.Init(); err != nil {
		return err
	}

	tokens := TokenData{
		XSRF:    xsrf,
		HHToken: hhtoken,
	}

	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}

	tokensPath := filepath.Join(s.configPath, tokensFile)
	return os.WriteFile(tokensPath, data, 0644)
}

func (s *Storage) LoadSchedule() (map[string]scheduler.ResumeSchedule, error) {
	schedulePath := filepath.Join(s.configPath, scheduleFile)
	
	if _, err := os.Stat(schedulePath); os.IsNotExist(err) {
		return make(map[string]scheduler.ResumeSchedule), nil
	}

	data, err := os.ReadFile(schedulePath)
	if err != nil {
		return nil, err
	}

	var schedules map[string]scheduler.ResumeSchedule
	if err := json.Unmarshal(data, &schedules); err != nil {
		return nil, err
	}

	if schedules == nil {
		schedules = make(map[string]scheduler.ResumeSchedule)
	}

	return schedules, nil
}

func (s *Storage) SaveSchedule(schedules map[string]scheduler.ResumeSchedule) error {
	if err := s.Init(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return err
	}

	schedulePath := filepath.Join(s.configPath, scheduleFile)
	return os.WriteFile(schedulePath, data, 0644)
}
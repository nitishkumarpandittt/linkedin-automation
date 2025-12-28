package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	LinkedIn struct {
		BaseURL string `yaml:"base_url"`
	} `yaml:"linkedin"`
	Search struct {
		Defaults struct {
			Title    string `yaml:"title"`
			Company  string `yaml:"company"`
			Location string `yaml:"location"`
			Keywords string `yaml:"keywords"`
		} `yaml:"defaults"`
	} `yaml:"search"`
	Limits struct {
		MaxConnectionsPerDay int `yaml:"max_connections_per_day"`
		MaxMessagesPerDay    int `yaml:"max_messages_per_day"`
		MaxProfilesPerSearch int `yaml:"max_profiles_per_search"`
	} `yaml:"limits"`
	Stealth struct {
		Headless           bool   `yaml:"headless"`
		EnableHumanMouse   bool   `yaml:"enable_human_mouse"`
		EnableRandomScroll bool   `yaml:"enable_random_scroll"`
		EnableTypeTypos    bool   `yaml:"enable_type_typos"`
		EnableHoverWander  bool   `yaml:"enable_hover_wander"`
		EnableBreaks       bool   `yaml:"enable_breaks"`
		UserAgent          string `yaml:"user_agent"`
		MinDelayMs         int    `yaml:"min_delay_ms"`
		MaxDelayMs         int    `yaml:"max_delay_ms"`
		ViewportWidthMin   int    `yaml:"viewport_width_min"`
		ViewportWidthMax   int    `yaml:"viewport_width_max"`
		ViewportHeightMin  int    `yaml:"viewport_height_min"`
		ViewportHeightMax  int    `yaml:"viewport_height_max"`
		ActiveStart        string `yaml:"active_start"`
		ActiveEnd          string `yaml:"active_end"`
	} `yaml:"stealth"`
	Templates struct {
		ConnectionNote string `yaml:"connection_note_template"`
		FollowUp       string `yaml:"follow_up_message_template"`
	} `yaml:"templates"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
}

func Load(path string) (*Config, error) {
	_ = godotenv.Load() // optional
	cfg := defaultConfig()
	if b, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnvOverrides(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaultConfig() Config {
	var cfg Config
	cfg.LinkedIn.BaseURL = "https://www.linkedin.com/"
	cfg.Limits.MaxConnectionsPerDay = 20
	cfg.Limits.MaxMessagesPerDay = 50
	cfg.Limits.MaxProfilesPerSearch = 200
	cfg.Stealth.Headless = false
	cfg.Stealth.EnableHumanMouse = true
	cfg.Stealth.EnableRandomScroll = true
	cfg.Stealth.EnableTypeTypos = true
	cfg.Stealth.EnableHoverWander = true
	cfg.Stealth.EnableBreaks = true
	cfg.Stealth.MinDelayMs = 120
	cfg.Stealth.MaxDelayMs = 900
	cfg.Stealth.ViewportWidthMin = 1280
	cfg.Stealth.ViewportWidthMax = 1680
	cfg.Stealth.ViewportHeightMin = 720
	cfg.Stealth.ViewportHeightMax = 1050
	cfg.Stealth.ActiveStart = "09:00"
	cfg.Stealth.ActiveEnd = "18:00"
	cfg.Database.Path = "linkedbot.db"
	cfg.Logging.Level = "info"
	cfg.Templates.ConnectionNote = "Hi {{Name}}, noticed your work at {{Company}} as {{Title}}—would love to connect."
	cfg.Templates.FollowUp = "Thanks for connecting, {{Name}}! If helpful, happy to share ideas around {{Keywords}}."
	return cfg
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LINKEDBOT_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("LINKEDBOT_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("LINKEDBOT_HEADLESS"); v == "1" || v == "true" {
		cfg.Stealth.Headless = true
	}
}

func validate(cfg *Config) error {
	if cfg.LinkedIn.BaseURL == "" {
		return errors.New("linkedin.base_url is required")
	}
	if cfg.Limits.MaxConnectionsPerDay <= 0 {
		return errors.New("limits.max_connections_per_day must be > 0")
	}
	if cfg.Limits.MaxMessagesPerDay <= 0 {
		return errors.New("limits.max_messages_per_day must be > 0")
	}
	if cfg.Limits.MaxProfilesPerSearch <= 0 {
		return errors.New("limits.max_profiles_per_search must be > 0")
	}
	if os.Getenv("LINKEDIN_EMAIL") == "" {
		return errors.New("LINKEDIN_EMAIL is required in env")
	}
	if os.Getenv("LINKEDIN_PASSWORD") == "" {
		return errors.New("LINKEDIN_PASSWORD is required in env")
	}
	return nil
}

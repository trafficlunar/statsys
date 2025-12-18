package internal

import (
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type ServiceConfig struct {
	Name             string `toml:"name"`
	Url              string `toml:"url"`
	LatencyThreshold int    `toml:"latency_threshold"`
}

type Config struct {
	Title               string          `toml:"title"`
	LinkText            string          `toml:"link_text"`
	LinkUrl             string          `toml:"link_url"`
	DefaultView         string          `toml:"default_view"`
	DefaultTheme        string          `toml:"default_theme"`
	EnableThemeSwitcher bool            `toml:"enable_theme_switcher"`
	EnableWatermark     bool            `toml:"enable_watermark"`
	Services            []ServiceConfig `toml:"services"`
}

var config Config

func LoadConfig() {
	configFile, err := os.ReadFile(*ConfigPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := toml.Unmarshal(configFile, &config); err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

	templateData.Title = config.Title
	templateData.LinkText = config.LinkText
	templateData.LinkUrl = config.LinkUrl
	templateData.EnableThemeSwitcher = config.EnableThemeSwitcher
	templateData.EnableWatermark = config.EnableWatermark
	templateData.Services = make([]Service, len(config.Services))
	for index, ser := range config.Services {
		// default to 1000ms
		latencyThreshold := int64(ser.LatencyThreshold)
		if latencyThreshold == 0 {
			latencyThreshold = 1000
		}

		service := Service{
			Name:             ser.Name,
			Url:              ser.Url,
			LatencyThreshold: latencyThreshold,
			Status:           "Unknown",
			MinuteTimeline:   make([]TimelineEntry, 30),
			HourTimeline:     make([]TimelineEntry, 24),
			DayTimeline:      make([]TimelineEntry, 30),
		}

		templateData.Services[index] = service
	}

	log.Printf("config loaded from '%s'", *ConfigPath)
}

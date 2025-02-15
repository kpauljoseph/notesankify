package config

import (
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	PDFSourceDir  string `yaml:"pdf_source_dir"`
	AnkiDeckName  string `yaml:"anki_deck_name"`
	FlashcardSize struct {
		Width  float64 `yaml:"width"`
		Height float64 `yaml:"height"`
	} `yaml:"flashcard_size"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.AnkiDeckName == "" {
		cfg.AnkiDeckName = "NotesAnkify"
	}
	if cfg.PDFSourceDir == "" {
		cfg.PDFSourceDir = "./pdfs"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}

	if cfg.FlashcardSize.Width == 0 || cfg.FlashcardSize.Height == 0 {
		cfg.FlashcardSize.Width = utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH
		cfg.FlashcardSize.Height = utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT
	}

	return &cfg, nil
}

package anki

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kpauljoseph/notesankify/internal/pdf"
)

const (
	DefaultAnkiConnectURL = "http://localhost:8765"
	DefaultModelName      = "Basic"
	MaxRetries            = 3
	RetryDelay            = 500 * time.Millisecond
)

type Service struct {
	ankiConnectURL string
	logger         *log.Logger
}

type AnkiConnectRequest struct {
	Action  string      `json:"action"`
	Version int         `json:"version"`
	Params  interface{} `json:"params"`
}

type Note struct {
	DeckName  string                 `json:"deckName"`
	ModelName string                 `json:"modelName"`
	Fields    map[string]string      `json:"fields"`
	Options   map[string]interface{} `json:"options"`
	Tags      []string               `json:"tags"`
}

func NewService(logger *log.Logger) *Service {
	return &Service{
		ankiConnectURL: DefaultAnkiConnectURL,
		logger:         logger,
	}
}

func (s *Service) CreateDeck(deckName string) error {
	s.logger.Printf("Creating deck: %s", deckName)
	request := AnkiConnectRequest{
		Action:  "createDeck",
		Version: 6,
		Params: map[string]string{
			"deck": deckName,
		},
	}

	_, err := s.sendRequest(request)
	return err
}

func (s *Service) AddFlashcard(deckName string, pair pdf.ImagePair) error {
	questionImage, err := s.readAndEncodeImage(pair.Question)
	if err != nil {
		return fmt.Errorf("failed to read question image: %w", err)
	}

	answerImage, err := s.readAndEncodeImage(pair.Answer)
	if err != nil {
		return fmt.Errorf("failed to read answer image: %w", err)
	}

	questionFileName := filepath.Base(pair.Question)
	answerFileName := filepath.Base(pair.Answer)

	if err := s.storeMediaFiles(map[string]string{
		questionFileName: questionImage,
		answerFileName:   answerImage,
	}); err != nil {
		return fmt.Errorf("failed to store media files: %w", err)
	}

	note := Note{
		DeckName:  deckName,
		ModelName: DefaultModelName,
		Fields: map[string]string{
			"Front": fmt.Sprintf("<img src=\"%s\">", questionFileName),
			"Back":  fmt.Sprintf("<img src=\"%s\">", answerFileName),
		},
		Options: map[string]interface{}{
			"allowDuplicate": false,
		},
		Tags: []string{"notesankify"},
	}

	request := AnkiConnectRequest{
		Action:  "addNote",
		Version: 6,
		Params: map[string]interface{}{
			"note": note,
		},
	}

	_, err = s.sendRequest(request)
	if err != nil {
		return fmt.Errorf("failed to add note: %w", err)
	}

	s.logger.Printf("Added flashcard: %s", questionFileName)
	return nil
}

func (s *Service) AddAllFlashcards(deckName string, pairs []pdf.ImagePair) error {
	var successCount, failCount int

	for _, pair := range pairs {
		if err := s.AddFlashcard(deckName, pair); err != nil {
			s.logger.Printf("Error adding flashcard: %v", err)
			failCount++
			continue
		}
		successCount++
	}

	if failCount > 0 {
		return fmt.Errorf("failed to add %d out of %d flashcards", failCount, len(pairs))
	}

	s.logger.Printf("Successfully added %d flashcards", successCount)

	return nil
}

func (s *Service) storeMediaFiles(files map[string]string) error {
	for filename, data := range files {
		request := AnkiConnectRequest{
			Action:  "storeMediaFile",
			Version: 6,
			Params: map[string]string{
				"filename": filename,
				"data":     data,
			},
		}

		_, err := s.sendRequest(request)
		if err != nil {
			return fmt.Errorf("failed to store media file %s: %w", filename, err)
		}
	}
	return nil
}

func (s *Service) readAndEncodeImage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (s *Service) sendRequest(req AnkiConnectRequest) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Printf("Retrying request (attempt %d/%d)...", attempt+1, MaxRetries)
			time.Sleep(RetryDelay)
		}

		reqBody, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		resp, err := http.Post(s.ankiConnectURL, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		var result struct {
			Error  *string         `json:"error"`
			Result json.RawMessage `json:"result"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		if result.Error != nil {
			lastErr = fmt.Errorf("anki error: %s", *result.Error)
			continue
		}

		return result.Result, nil
	}

	return nil, fmt.Errorf("after %d attempts: %v", MaxRetries, lastErr)
}

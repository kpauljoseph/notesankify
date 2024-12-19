package anki

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"io"
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
	logger         *logger.Logger
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

type NoteInfo struct {
	NoteId    int      `json:"noteId"`
	ModelName string   `json:"modelName"`
	Fields    Fields   `json:"fields"`
	Tags      []string `json:"tags"`
}

type Fields struct {
	Front struct {
		Value string `json:"value"`
		Order int    `json:"order"`
	} `json:"Front"`
	Back struct {
		Value string `json:"value"`
		Order int    `json:"order"`
	} `json:"Back"`
}

func NewService(logger *logger.Logger) *Service {
	return &Service{
		ankiConnectURL: DefaultAnkiConnectURL,
		logger:         logger,
	}
}

func (s *Service) CheckConnection() error {
	request := AnkiConnectRequest{
		Action:  "version",
		Version: 6,
		Params:  map[string]interface{}{},
	}

	_, err := s.sendRequest(request)
	if err != nil {
		s.logger.Printf("Error sending request to Anki: %v", err)
		return fmt.Errorf("could not connect to Anki. Please ensure:\n" +
			"1. Anki is running https://apps.ankiweb.net/#download\n" +
			"2. AnkiConnect add-on is installed (code: 2055492159) https://ankiweb.net/shared/info/2055492159\n" +
			"3. Anki has been restarted after installing AnkiConnect")
	}

	return nil
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

func (s *Service) findExistingNote(front, back string) (int, error) {
	s.logger.Debug("Searching for existing note...")

	request := AnkiConnectRequest{
		Action:  "findNotes",
		Version: 6,
		Params: map[string]interface{}{
			"query": fmt.Sprintf("deck:%s", "\"*\""),
		},
	}

	result, err := s.sendRequest(request)
	if err != nil {
		return 0, fmt.Errorf("failed to search notes: %w", err)
	}

	var noteIds []int
	if err := json.Unmarshal(result, &noteIds); err != nil {
		return 0, fmt.Errorf("failed to parse note IDs: %w", err)
	}

	s.logger.Printf("Found %d total notes to check", len(noteIds))

	if len(noteIds) == 0 {
		return 0, nil
	}

	// Get info for found notes
	request = AnkiConnectRequest{
		Action:  "notesInfo",
		Version: 6,
		Params: map[string]interface{}{
			"notes": noteIds,
		},
	}

	result, err = s.sendRequest(request)
	if err != nil {
		return 0, fmt.Errorf("failed to get notes info: %w", err)
	}

	var notes []NoteInfo
	if err := json.Unmarshal(result, &notes); err != nil {
		return 0, fmt.Errorf("failed to parse notes info: %w", err)
	}

	// Compare content with detailed logging
	for _, note := range notes {
		if note.Fields.Front.Value == front && note.Fields.Back.Value == back {
			s.logger.Info("Found exact match with note ID: %d", note.NoteId)
			s.logger.Debug("Existing front: %s", truncateString(note.Fields.Front.Value, 100))
			s.logger.Debug("New front: %s", truncateString(front, 100))
			s.logger.Debug("Existing back: %s", truncateString(note.Fields.Back.Value, 100))
			s.logger.Debug("New back: %s", truncateString(back, 100))
			return note.NoteId, nil
		}
	}

	s.logger.Printf("No matching note found")
	return 0, nil
}

func (s *Service) AddFlashcard(deckName string, pair pdf.ImagePair) error {
	s.logger.Printf("Processing new flashcard for deck: %s", deckName)
	s.logger.Printf("Question image: %s Answer image: %s", pair.Question, pair.Answer)

	questionImage, err := s.readAndEncodeImage(pair.Question)
	if err != nil {
		return fmt.Errorf("failed to read question image: %w", err)
	}

	answerImage, err := s.readAndEncodeImage(pair.Answer)
	if err != nil {
		return fmt.Errorf("failed to read answer image: %w", err)
	}

	// Generate content hash
	contentHash, err := FlashcardHash(pair.Question, pair.Answer)
	if err != nil {
		return fmt.Errorf("failed to generate content hash: %w", err)
	}
	s.logger.Printf("Generated content hash: %s", contentHash)

	front := fmt.Sprintf("<img src=\"%s\">", filepath.Base(pair.Question))
	back := fmt.Sprintf("<img src=\"%s\">", filepath.Base(pair.Answer))

	s.logger.Debug("Checking for existing note...")
	existingNoteId, err := s.findExistingNote(front, back)
	if err != nil {
		s.logger.Printf("Warning: failed to check for existing note: %v", err)
	} else if existingNoteId != 0 {
		s.logger.Printf("Duplicate found - skipping flashcard with hash: %s", contentHash)
		return nil
	}

	s.logger.Printf("No duplicate found, proceeding with adding new card")

	if err := s.storeMediaFiles(map[string]string{
		filepath.Base(pair.Question): questionImage,
		filepath.Base(pair.Answer):   answerImage,
	}); err != nil {
		return fmt.Errorf("failed to store media files: %w", err)
	}

	note := Note{
		DeckName:  deckName,
		ModelName: DefaultModelName,
		Fields: map[string]string{
			"Front": front,
			"Back":  back,
		},
		Options: map[string]interface{}{
			"allowDuplicate": false,
		},
		Tags: []string{"notesankify", fmt.Sprintf("hash:%s", contentHash)},
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

	s.logger.Printf("Successfully added new flashcard with hash: %s", contentHash)
	return nil
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
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

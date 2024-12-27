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
	"strings"
	"time"

	"github.com/kpauljoseph/notesankify/internal/pdf"
)

const (
	DefaultAnkiConnectURL = "http://localhost:8765"
	NotesAnkifyModelName  = "NotesAnkify"
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
	Hash struct {
		Value string `json:"value"`
		Order int    `json:"order"`
	} `json:"Hash"`
}

type ProcessingReport struct {
	TotalProcessed  int
	AddedCount      int
	SkippedCount    int
	SkippedCards    []SkippedCardInfo
	ProcessedPDFs   int
	TotalFlashcards int
	StartTime       time.Time
	EndTime         time.Time
}

type SkippedCardInfo struct {
	DeckName   string
	Hash       string
	PageNumber int
}

func NewService(logger *logger.Logger) *Service {
	return &Service{
		ankiConnectURL: DefaultAnkiConnectURL,
		logger:         logger,
	}
}

func (s *Service) ensureModelExists() error {
	request := AnkiConnectRequest{
		Action:  "modelNames",
		Version: ANKI_CONNECT_VERSION,
		Params:  map[string]interface{}{},
	}

	result, err := s.sendRequest(request)
	if err != nil {
		return fmt.Errorf("failed to get models: %w", err)
	}

	var modelNames []string
	if err := json.Unmarshal(result, &modelNames); err != nil {
		return fmt.Errorf("failed to parse model names: %w", err)
	}

	for _, name := range modelNames {
		if name == NotesAnkifyModelName {
			s.logger.Debug("NotesAnkify model already exists")
			return nil
		}
	}

	createRequest := AnkiConnectRequest{
		Action:  "createModel",
		Version: ANKI_CONNECT_VERSION,
		Params: map[string]interface{}{
			"modelName": NotesAnkifyModelName,
			"inOrderFields": []string{
				"Front",
				"Back",
				"Hash",
			},
			"css": `.card {
                font-family: arial;
                font-size: 20px;
                text-align: center;
                color: black;
                background-color: white;
            }
            .hash { display: none; }`,
			"cardTemplates": []map[string]interface{}{
				{
					"Name": "Card 1",
					"Front": `{{Front}}
                        <div class="hash">{{Hash}}</div>`,
					"Back": `{{FrontSide}}
                        <hr id="answer">
                        {{Back}}`,
				},
			},
		},
	}

	_, err = s.sendRequest(createRequest)
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}

	s.logger.Info("Created NotesAnkify model")
	return nil
}

func (s *Service) CheckConnection() error {
	request := AnkiConnectRequest{
		Action:  "version",
		Version: ANKI_CONNECT_VERSION,
		Params:  map[string]interface{}{},
	}

	_, err := s.sendRequest(request)
	if err != nil {
		s.logger.Info("Error sending request to Anki: %v", err)
		return fmt.Errorf("could not connect to Anki. Please ensure:\n" +
			"1. Anki is running https://apps.ankiweb.net/#download\n" +
			"2. AnkiConnect add-on is installed (code: 2055492159) https://ankiweb.net/shared/info/2055492159\n" +
			"3. Anki has been restarted after installing AnkiConnect")
	}

	return nil
}

func (s *Service) CreateDeck(deckName string) error {
	s.logger.Info("Creating deck: %s", deckName)
	request := AnkiConnectRequest{
		Action:  "createDeck",
		Version: ANKI_CONNECT_VERSION,
		Params: map[string]string{
			"deck": deckName,
		},
	}

	_, err := s.sendRequest(request)
	return err
}

func (s *Service) findExistingNoteByHash(hash string) (int, error) {
	request := AnkiConnectRequest{
		Action:  "findNotes",
		Version: ANKI_CONNECT_VERSION,
		Params: map[string]interface{}{
			"query": fmt.Sprintf("Hash:%s", hash),
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

	if len(noteIds) > 0 {
		return noteIds[0], nil
	}

	return 0, nil
}

func (s *Service) AddFlashcard(deckName string, pair pdf.ImagePair, pageNum int, report *ProcessingReport) error {
	report.TotalProcessed++

	s.logger.Debug("Processing new flashcard for deck: %s", deckName)
	s.logger.Debug("Question image: %s", pair.Question)
	s.logger.Debug("Answer image: %s", pair.Answer)
	s.logger.Debug("Using content hash: %s", pair.Hash)

	// Check for existing note with same hash
	existingNoteId, err := s.findExistingNoteByHash(pair.Hash)
	if err != nil {
		s.logger.Debug("Warning: failed to check for existing note: %v", err)
	} else if existingNoteId != 0 {
		s.logger.Info("Skipping duplicate flashcard with hash: %s", pair.Hash)
		report.SkippedCount++
		report.SkippedCards = append(report.SkippedCards,
			SkippedCardInfo{
				DeckName:   deckName,
				Hash:       pair.Hash,
				PageNumber: pageNum,
			})
		return nil
	}

	questionImage, err := s.readAndEncodeImage(pair.Question)
	if err != nil {
		return fmt.Errorf("failed to read question image: %w", err)
	}

	answerImage, err := s.readAndEncodeImage(pair.Answer)
	if err != nil {
		return fmt.Errorf("failed to read answer image: %w", err)
	}

	if err := s.storeMediaFiles(map[string]string{
		filepath.Base(pair.Question): questionImage,
		filepath.Base(pair.Answer):   answerImage,
	}); err != nil {
		return fmt.Errorf("failed to store media files: %w", err)
	}

	note := Note{
		DeckName:  deckName,
		ModelName: NotesAnkifyModelName,
		Fields: map[string]string{
			"Front": fmt.Sprintf("<img src=\"%s\">", filepath.Base(pair.Question)),
			"Back":  fmt.Sprintf("<img src=\"%s\">", filepath.Base(pair.Answer)),
			"Hash":  pair.Hash,
		},
		Options: map[string]interface{}{
			"allowDuplicate": false,
		},
		Tags: []string{"notesankify", getDeckNameUnderscoreSeparatedForTag(deckName)},
	}

	request := AnkiConnectRequest{
		Action:  "addNote",
		Version: ANKI_CONNECT_VERSION,
		Params: map[string]interface{}{
			"note": note,
		},
	}

	_, err = s.sendRequest(request)
	if err != nil {
		return fmt.Errorf("failed to add note: %w", err)
	}

	s.logger.Debug("Successfully added new flashcard with hash: %s", pair.Hash)
	report.AddedCount++
	return nil
}

func (s *Service) AddAllFlashcards(deckName string, pairs []pdf.ImagePair, pageNumbers []int, report *ProcessingReport) error {
	var successCount, failCount int

	if err := s.ensureModelExists(); err != nil {
		return fmt.Errorf("failed to ensure model exists: %w", err)
	}

	for index, pair := range pairs {
		if err := s.AddFlashcard(deckName, pair, pageNumbers[index], report); err != nil {
			s.logger.Debug("Error adding flashcard: %v", err)
			failCount++
			continue
		}
		successCount++
	}

	if failCount > 0 {
		return fmt.Errorf("failed to add %d out of %d flashcards", failCount, len(pairs))
	}

	s.logger.Debug("Successfully processed %d flashcards\n\n\n\n\n", successCount)

	return nil
}

func (s *Service) storeMediaFiles(files map[string]string) error {
	for filename, data := range files {
		request := AnkiConnectRequest{
			Action:  "storeMediaFile",
			Version: ANKI_CONNECT_VERSION,
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
			s.logger.Info("Retrying request (attempt %d/%d)...", attempt+1, MaxRetries)
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

func getDeckNameUnderscoreSeparatedForTag(deckName string) string {
	return strings.ReplaceAll(strings.TrimSpace(deckName), " ", "_")
}

func (r *ProcessingReport) TimeTaken() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}
func (r *ProcessingReport) Print(logger *logger.Logger) {
	fmt.Printf("\n\n\nProcessing Report:")
	fmt.Printf("\n-------------------------------------------------------------\n")
	fmt.Printf("\nTotal PDFs Processed: %d", r.ProcessedPDFs)
	fmt.Printf("\nTotal Flashcards Found: %d", r.TotalFlashcards)
	fmt.Printf("\nCards Added: %d", r.AddedCount)
	fmt.Printf("\nCards Skipped (Duplicates): %d", r.SkippedCount)
	fmt.Printf("\nTime Taken: %v", r.TimeTaken())

	if r.SkippedCount > 0 {
		fmt.Printf("\n\n\nSkipped Cards:")
		fmt.Printf("\n-------------------------------------------------------------\n")
		for _, card := range r.SkippedCards {
			fmt.Printf("- %s (Page %d, Hash:%s)",
				card.DeckName,
				card.PageNumber,
				card.Hash)
		}
	}
}

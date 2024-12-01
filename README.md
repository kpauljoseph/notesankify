# NotesAnkify

Convert PDF notes to Anki flashcards automatically. This tool monitors a directory for PDFs containing flashcards, processes them based on specified page dimensions, and creates/updates Anki decks while maintaining learning progress.

## Features

- PDF flashcard detection based on page dimensions
- Automatic Anki deck creation
- Duplicate prevention
- Change detection for modified cards
- OneDrive integration (planned)

## Prerequisites

- Go 1.21 or later
- SQLite (for metadata storage)
- `go-fitz` for PDF processing
- `goanki` for Anki integration

## Project Structure

```
notesankify/
├── cmd/
│   └── notesankify/
│       └── main.go
├── internal/
│   ├── pdf/
│   │   ├── processor.go
│   │   └── processor_test.go
│   ├── anki/
│   │   ├── deck.go
│   │   └── deck_test.go
│   ├── storage/
│   │   ├── metadata.go
│   │   └── metadata_test.go
│   └── config/
│       └── config.go
├── pkg/
│   └── models/
│       └── flashcard.go
├── tests/
│   └── integration/
├── .gitignore
├── go.mod
├── go.sum
├── README.md
├── CHANGELOG.md
└── Makefile
```

## Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/kpauljoseph/notesankify.git
   cd notesankify
   ```

2. Initialize Go module:
   ```bash
   go mod init github.com/kpauljoseph/notesankify
   ```

3. Install dependencies:
   ```bash
   go mod tidy
   ```

4. Build the project:
   ```bash
   make build
   ```

5. Run tests:
   ```bash
   make test
   ```

## Development

### Making Changes

1. Create a new branch for your feature:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and update CHANGELOG.md
3. Run tests and linting:
   ```bash
   make check
   ```

4. Commit your changes:
   ```bash
   git commit -m "feat: description of your changes"
   ```

### Testing

We use Go's built-in testing framework with testify for assertions:

```bash
make test        # Run unit tests
make test-int    # Run integration tests
make test-all    # Run all tests with coverage
```

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
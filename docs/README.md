<div align="left">
  <img src="docs/images/NotesAnkify-icon-256.png" alt="NotesAnkify" width="128" height="128">
</div>

## NotesAnkify Documentation

Turn your PDF notes into Anki flashcards automatically. Study smarter with spaced repetition!

// INSERT IMAGE - A side-by-side comparison showing a PDF note on the left with question/answer sections and the resulting Anki flashcard on the right.

## Table of Contents
- [Why NotesAnkify?](#why-notesankify)
- [Quick Start](#quick-start)
- [Installation](#installation)
    - [Windows](#windows)
    - [macOS](#macos)
    - [Linux](#linux)
- [Creating Flashcards](#creating-flashcards)
    - [Using Question/Answer Markers](#1-using-questionanswer-markers)
    - [Using Standard Dimensions](#2-using-standard-dimensions)
    - [Simple Top/Bottom Split](#3-simple-topbottom-split)
- [Processing Modes](#processing-modes)
    - [Markers + Dimensions Mode](#1-markers--dimensions-mode-most-strict)
    - [Markers Only Mode](#2-markers-only-mode-flexible-size)
    - [Dimensions Only Mode](#3-dimensions-only-mode-standard-size)
    - [Process All Pages Mode](#4-process-all-pages-mode-most-flexible)
- [Deck Organization](#deck-organization)
- [Advanced Features](#advanced-features)
    - [Duplicate Detection & Smart Updating](#duplicate-detection--smart-updating)
    - [Output Directory](#output-directory)
    - [Processing Report](#processing-report)
- [Troubleshooting](#troubleshooting)
    - [Common Issues](#common-issues)
    - [Finding Log Files](#finding-log-files)
- [FAQ](#faq)
- [Need Help?](#need-help)

## Why NotesAnkify?

Taking notes and creating flashcards are essential for effective studying, but maintaining them separately is time-consuming. NotesAnkify bridges this gap by:

- Converting your PDF notes directly into Anki flashcards
- Preserving your existing note-taking workflow
- Supporting multiple note formats and apps
- Preventing duplicate flashcards automatically
- Working with both handwritten and typed notes

## Quick Start

1. **Install Prerequisites**
    - Download [Anki Desktop](https://apps.ankiweb.net/)
    - Install [AnkiConnect](https://ankiweb.net/shared/info/2055492159) add-on
    - Download NotesAnkify for your platform (See [Installation Guide](#installation))

2. **Format Your Notes**
   Choose any of these methods:
    - Add QUESTION/ANSWER markers
    - Use standard dimensions (455.04 × 587.52 points)
    - Split pages into top (question) and bottom (answer)

3. **Convert to Flashcards**
    - Start Anki
    - Launch NotesAnkify
    - Select PDF directory
    - Choose processing mode
    - Click "Process and Send to Anki"

// INSERT IMAGE - Screenshot of NotesAnkify's main interface with key areas highlighted and numbered according to the steps above

## Installation

### Windows
1. Download the ZIP file for your system (AMD64/ARM64)
2. Extract the ZIP file to your desired location
3. Run NotesAnkify.exe

// INSERT IMAGE - Screenshot showing Windows security warning with "More info" and "Run anyway" buttons highlighted

### macOS
1. Download the DMG file
2. Open the DMG file
3. Drag NotesAnkify to your Applications folder 
4. Right-click and select "Open" on first launch

// INSERT IMAGE - Screenshot showing macOS Gatekeeper dialog with right-click menu open

### Linux
1. Download the tar.xz file 
2. Extract using: `tar xf NotesAnkify-linux-*.tar.xz`
3. Run the NotesAnkify executable.

## Creating Flashcards

You can create flashcards in three ways:

### 1. Using Question/Answer Markers

Add "QUESTION" and "ANSWER" text to your notes:

QUESTION
What is photosynthesis?

ANSWER
Process where plants convert...

// INSERT IMAGE - Example of a note page with QUESTION/ANSWER markers highlighted

### 2. Using Standard Dimensions

Create pages with these exact dimensions:
- Width: 455.04 points
- Height: 587.52 points
- Question on top half
- Answer on bottom half

// INSERT IMAGE - Template showing standard dimensions with measurements labeled

### 3. Simple Top/Bottom Split

Any page can be split into:
- Top half → Question
- Bottom half → Answer

// INSERT IMAGE - Example of a regular page split into question (top) and answer (bottom)

## Processing Modes

NotesAnkify offers four ways to process your notes:

### 1. Markers + Dimensions Mode (Most Strict)
- Requires QUESTION/ANSWER markers
- Must match standard dimensions
- Best for consistent flashcard creation
- Perfect when using templates

### 2. Markers Only Mode (Flexible Size)
- Only checks for QUESTION/ANSWER markers
- Any page size accepted
- Good for mixed-size documents
- Best when you can't control page size

### 3. Dimensions Only Mode (Standard Size)
- Only checks page dimensions
- Splits page into top/bottom
- No markers needed
- Ideal for template-based notes

### 4. Process All Pages Mode (Most Flexible)
- Processes every page
- Splits each page in half
- No formatting requirements
- Quick conversion for simple notes

// INSERT IMAGE - Side-by-side comparison showing example pages that work with each mode

## Deck Organization

Your PDFs are organized into Anki decks following your folder structure:

PDFs/
└── Biology/
├── Chapter1.pdf
└── Chapter2.pdf
└── Chemistry/
└── Notes.pdf

Results in:
- Biology::Chapter1
- Biology::Chapter2
- Chemistry::Notes

// INSERT IMAGE - Screenshot showing the folder structure on the left and resulting Anki deck organization on the right

## Advanced Features

### Duplicate Detection & Smart Updating

NotesAnkify uses a technique called "hashing" to manage flashcards intelligently. Think of a hash as a unique fingerprint for each flashcard - even a tiny change will create a different fingerprint.

// INSERT IMAGE - Visual showing how changing content creates different hashes. Example: Two similar flashcards with one small difference, showing different hash values

#### How it Works
1. When a flashcard is processed, NotesAnkify looks at every pixel in the image
2. It combines all the color values into a unique string of characters (the hash)
3. This hash is stored with the flashcard in Anki
4. When processing PDFs again:
    - Same content = Same hash = Skip (prevent duplicate)
    - Changed content = New hash = Update card

#### Benefits
- You can keep flashcards in multiple PDFs without duplicates
- Modified flashcards are automatically updated
- Original cards are preserved if content hasn't changed
- You don't need to track which cards you've already converted

// INSERT IMAGE - Flowchart showing how the same card in multiple PDFs only creates one Anki card

#### Example
Imagine you have the same flashcard in two places:
1. Your chapter notes
2. Your exam review guide

NotesAnkify will:
- Recognize they're the same card
- Only create it once in Anki
- Update it if you make changes in either place

### Output Directory
Save processed flashcard images to:
- Review conversion results
- Debug any issues
- Keep a backup of generated cards

### Processing Report
After conversion, you'll see:
- Total PDFs processed
- Number of flashcards created
- Processing time
- Log file location

## Troubleshooting

### Common Issues

#### Cannot Connect to Anki
1. Ensure Anki is running
2. Verify AnkiConnect is installed
3. Restart Anki and try again

#### No Flashcards Created
1. Check PDF formatting 
2. Verify chosen processing mode 
3. Review processing report

#### Access Denied Errors
1. Run as administrator (Windows)
2. Check folder permissions 
3. Verify write access to output directory

### Finding Log Files

Logs are saved in:
- Windows: `%USERPROFILE%\notesankify-logs\`
- macOS: `~/notesankify-logs/`
- Linux: `~/notesankify-logs/`

Include these logs when reporting issues.

## FAQ

**Q: Does it work with handwritten notes?**  
A: Yes! NotesAnkify works with any PDF content, including handwritten notes, typed text, or diagrams.

**Q: Will my existing Anki flashcards be affected?**  
A: No. NotesAnkify safely adds new flashcards without modifying existing ones.

**Q: What note-taking apps are supported?**  
A: Any app that can export to PDF works, including:
- GoodNotes
- Notability
- OneNote
- Nebo
- Any PDF-capable app

## Need Help?

- [Read the Documentation](https://notesankify.com/docs)
- [Report an Issue](https://github.com/kpauljoseph/notesankify/issues)
- [Join Discussions](https://github.com/kpauljoseph/notesankify/discussions)
- [Check the Wiki](https://github.com/kpauljoseph/notesankify/wiki)
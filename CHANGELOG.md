# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- OneDrive sync capability for automatic PDF download from Microsoft OneDrive
- OAuth2 device code authentication for OneDrive
- New CLI flags: `--sync-onedrive`, `--onedrive-auth`, `--onedrive-clear-auth`
- OneDrive configuration section in config.yaml
- Comprehensive OneDrive setup documentation (docs/ONEDRIVE_SETUP.md)
- Smart sync with file skip detection (avoids re-downloading unchanged files)
- Recursive PDF discovery in OneDrive folders
- Sync statistics and error reporting
- Microsoft Graph SDK integration for OneDrive API access

### Changed
- Updated config package to support OneDrive settings
- Enhanced CLI to support OneDrive sync workflow
- Updated main documentation with OneDrive sync information

## [1.0.0] - 2025-02-07

### Added
- 1.0.0 launch!

### Changed
- N/A

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- N/A

## [0.1.0] - 2024-12-30

### Added
- Initial release of NotesAnkify
- PDF to Anki flashcard conversion
- Support for multiple page processing modes
- Automatic deck creation and organization
- GUI interface
- Support for Mac/Windows/Linux

[Unreleased]: https://github.com/kpauljoseph/notesankify/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/kpauljoseph/notesankify/releases/tag/v0.1.0
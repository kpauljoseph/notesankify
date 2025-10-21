package onedrive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kpauljoseph/notesankify/pkg/logger"
)

// SyncService handles OneDrive synchronization
type SyncService struct {
	config *Config
	auth   *Authenticator
	client *Client
	logger *logger.Logger
}

// NewSyncService creates a new sync service
func NewSyncService(config *Config, log *logger.Logger) (*SyncService, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("OneDrive sync is not enabled")
	}

	auth, err := NewAuthenticator(config, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %w", err)
	}

	client := NewClient(auth, log)

	return &SyncService{
		config: config,
		auth:   auth,
		client: client,
		logger: log,
	}, nil
}

// Authenticate performs the authentication flow
func (s *SyncService) Authenticate(ctx context.Context) error {
	return s.auth.Authenticate(ctx)
}

// Sync downloads PDFs from OneDrive to local directory
func (s *SyncService) Sync(ctx context.Context, localDir string) (*SyncStats, error) {
	stats := &SyncStats{
		StartTime: time.Now(),
	}

	s.logger.Info("Starting OneDrive sync...")
	s.logger.Info("OneDrive folder: %s", s.config.FolderPath)
	s.logger.Info("Local directory: %s", localDir)

	// Ensure local directory exists
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return stats, fmt.Errorf("failed to create local directory: %w", err)
	}

	// List all PDF files from OneDrive
	files, err := s.client.ListFilesRecursive(ctx, s.config.FolderPath)
	if err != nil {
		return stats, fmt.Errorf("failed to list OneDrive files: %w", err)
	}

	s.logger.Info("Found %d PDF files to sync", len(files))

	// Download each file
	for i, file := range files {
		s.logger.Debug("[%d/%d] Processing: %s", i+1, len(files), file.Name)

		// Build local file path
		localPath := filepath.Join(localDir, file.Name)

		// Check if file exists and has same size
		if s.shouldSkipFile(localPath, file) {
			s.logger.Debug("Skipping (already up-to-date): %s", file.Name)
			stats.FilesSkipped++
			continue
		}

		// Download the file
		if err := s.client.DownloadFile(ctx, file, localPath); err != nil {
			s.logger.Info("Failed to download %s: %v", file.Name, err)
			stats.AddError(fmt.Errorf("download failed for %s: %w", file.Name, err))
			continue
		}

		stats.FilesDownloaded++
		stats.BytesDownloaded += file.Size
		s.logger.Info("Downloaded: %s", file.Name)
	}

	stats.EndTime = time.Now()

	// Print summary
	s.logger.Info("\nSync completed:")
	s.logger.Info("- Files downloaded: %d", stats.FilesDownloaded)
	s.logger.Info("- Files skipped: %d", stats.FilesSkipped)
	s.logger.Info("- Bytes downloaded: %d (%.2f MB)", stats.BytesDownloaded, float64(stats.BytesDownloaded)/(1024*1024))
	s.logger.Info("- Duration: %v", stats.Duration())
	if len(stats.Errors) > 0 {
		s.logger.Info("- Errors: %d", len(stats.Errors))
	}

	return stats, nil
}

// shouldSkipFile checks if a file should be skipped (already exists with same size)
func (s *SyncService) shouldSkipFile(localPath string, remoteFile DriveItem) bool {
	info, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return false // File doesn't exist, don't skip
	}
	if err != nil {
		s.logger.Debug("Error checking file %s: %v", localPath, err)
		return false // Error checking, download anyway
	}

	// Skip if size matches and local file is newer or same age
	sizeMatches := info.Size() == remoteFile.Size
	timeMatches := !info.ModTime().Before(remoteFile.ModifiedDateTime)

	return sizeMatches && timeMatches
}

// ClearAuth removes stored authentication
func (s *SyncService) ClearAuth() error {
	return s.auth.ClearToken()
}

// IsAuthenticated checks if user is authenticated
func (s *SyncService) IsAuthenticated() bool {
	if s.auth.cred == nil {
		return false
	}

	// Try to load token and check if it's valid
	token, err := s.auth.loadToken()
	if err != nil {
		return false
	}

	return !s.auth.isTokenExpired(token)
}

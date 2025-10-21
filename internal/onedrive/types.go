package onedrive

import (
	"time"
)

// Config holds OneDrive configuration
type Config struct {
	ClientID     string `yaml:"client_id"`
	TenantID     string `yaml:"tenant_id"`     // defaults to "common"
	FolderPath   string `yaml:"folder_path"`   // OneDrive folder path to sync
	SyncInterval int    `yaml:"sync_interval"` // minutes, 0 = manual sync only
	Enabled      bool   `yaml:"enabled"`
}

// TokenStore represents the stored authentication token
type TokenStore struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// DriveItem represents a OneDrive file or folder
type DriveItem struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Size             int64     `json:"size"`
	CreatedDateTime  time.Time `json:"createdDateTime"`
	ModifiedDateTime time.Time `json:"lastModifiedDateTime"`
	DownloadURL      string    `json:"@microsoft.graph.downloadUrl"`
	Folder           *struct{} `json:"folder,omitempty"`
	File             *struct {
		MimeType string `json:"mimeType"`
	} `json:"file,omitempty"`
}

// DriveItemList represents a list of drive items
type DriveItemList struct {
	Value    []DriveItem `json:"value"`
	NextLink string      `json:"@odata.nextLink,omitempty"`
}

// SyncStats tracks sync statistics
type SyncStats struct {
	FilesDownloaded int
	FilesSkipped    int
	FilesDeleted    int
	BytesDownloaded int64
	StartTime       time.Time
	EndTime         time.Time
	Errors          []error
}

// AddError adds an error to sync stats
func (s *SyncStats) AddError(err error) {
	s.Errors = append(s.Errors, err)
}

// Duration returns the sync duration
func (s *SyncStats) Duration() time.Duration {
	return s.EndTime.Sub(s.StartTime)
}

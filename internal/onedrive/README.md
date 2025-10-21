# OneDrive Package

This package provides OneDrive integration for NotesAnkify, enabling automatic synchronization of PDF files from Microsoft OneDrive.

## Features

- OAuth2 authentication using device code flow (user-friendly for CLI apps)
- Automatic token storage and refresh
- Recursive PDF file discovery in OneDrive folders
- Smart sync (skip files that are already up-to-date)
- Detailed sync statistics and error reporting

## Components

### auth.go
Handles OAuth2 authentication with Microsoft Graph API using device code flow.

**Key Features:**
- Device code authentication (no browser popup required)
- Secure token storage in user home directory
- Token expiration checking
- Easy credential clearing

### client.go
Microsoft Graph API client for OneDrive operations.

**Key Features:**
- List files and folders in OneDrive
- Recursive file listing with filtering
- File download with progress
- Pagination support for large directories

### sync.go
High-level sync service that orchestrates authentication and file synchronization.

**Key Features:**
- Automatic PDF detection and filtering
- Skip unchanged files (same size and modification time)
- Comprehensive sync statistics
- Error collection without stopping the sync process

### types.go
Type definitions for configuration, tokens, and API responses.

## Usage Example

```go
import (
    "context"
    "github.com/kpauljoseph/notesankify/internal/onedrive"
    "github.com/kpauljoseph/notesankify/pkg/logger"
)

// Configure OneDrive
config := &onedrive.Config{
    Enabled:      true,
    ClientID:     "your-azure-app-client-id",
    TenantID:     "common",
    FolderPath:   "Documents/Notes",
    SyncInterval: 0,
}

log := logger.New()

// Create sync service
syncService, err := onedrive.NewSyncService(config, log)
if err != nil {
    log.Fatal("Failed to create sync service: %v", err)
}

// Authenticate (first time)
if err := syncService.Authenticate(context.Background()); err != nil {
    log.Fatal("Authentication failed: %v", err)
}

// Sync PDFs
stats, err := syncService.Sync(context.Background(), "./local-pdfs")
if err != nil {
    log.Fatal("Sync failed: %v", err)
}

log.Info("Downloaded %d files", stats.FilesDownloaded)
```

## Authentication Flow

1. User runs `notesankify --onedrive-auth`
2. Device code flow is initiated
3. User opens browser and enters code
4. User signs in with Microsoft account
5. Token is saved to `~/.notesankify/onedrive_token.json`
6. Future syncs use the saved token automatically

## Security

- Uses Microsoft's official Azure SDK
- Tokens are stored securely in user's home directory
- Only requests read permissions (`Files.Read`, `Files.Read.All`)
- Supports token refresh for long-running operations

## API Endpoints Used

- `GET /me/drive/root/children` - List root folder
- `GET /me/drive/root:/{path}:/children` - List specific folder
- `GET /me/drive/items/{id}` - Get file metadata
- Download URL is provided in the item metadata

## Error Handling

The sync service collects errors without stopping:
```go
stats, err := syncService.Sync(ctx, localDir)
if err != nil {
    // Fatal error (e.g., authentication failed)
    return err
}

// Check for non-fatal errors
if len(stats.Errors) > 0 {
    for _, err := range stats.Errors {
        log.Info("Sync error: %v", err)
    }
}
```

## Configuration

See `config.yaml` in the project root for configuration options:

```yaml
onedrive:
  enabled: true
  client_id: "your-client-id"
  tenant_id: "common"
  folder_path: "Documents/Notes"
  sync_interval: 0
```

## Dependencies

- `github.com/Azure/azure-sdk-for-go/sdk/azidentity` - Azure authentication
- `github.com/Azure/azure-sdk-for-go/sdk/azcore` - Azure core functionality
- `github.com/microsoftgraph/msgraph-sdk-go-core` - Microsoft Graph API client

## Future Enhancements

- [ ] Automatic periodic sync based on `sync_interval`
- [ ] Selective file sync (exclude patterns)
- [ ] Bi-directional sync (upload local changes)
- [ ] Progress callbacks for large downloads
- [ ] Concurrent downloads for better performance
- [ ] GUI integration for authentication

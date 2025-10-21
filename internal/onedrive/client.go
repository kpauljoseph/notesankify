package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/kpauljoseph/notesankify/pkg/logger"
)

const (
	graphAPIBaseURL = "https://graph.microsoft.com/v1.0"
)

// Client represents a OneDrive API client
type Client struct {
	auth      *Authenticator
	logger    *logger.Logger
	httpClient *http.Client
}

// NewClient creates a new OneDrive client
func NewClient(auth *Authenticator, log *logger.Logger) *Client {
	return &Client{
		auth:       auth,
		logger:     log,
		httpClient: &http.Client{},
	}
}

// ListFiles lists all files in a OneDrive folder path
func (c *Client) ListFiles(ctx context.Context, folderPath string) ([]DriveItem, error) {
	c.logger.Debug("Listing files in OneDrive folder: %s", folderPath)

	var allItems []DriveItem
	nextLink := c.buildListURL(folderPath)

	for nextLink != "" {
		items, next, err := c.listFilesPage(ctx, nextLink)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
		nextLink = next
	}

	c.logger.Debug("Found %d items in OneDrive folder", len(allItems))
	return allItems, nil
}

// listFilesPage retrieves a single page of files
func (c *Client) listFilesPage(ctx context.Context, url string) ([]DriveItem, string, error) {
	token, err := c.auth.GetAccessToken(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("OneDrive API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result DriveItemList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Value, result.NextLink, nil
}

// ListFilesRecursive lists all PDF files recursively in a folder
func (c *Client) ListFilesRecursive(ctx context.Context, folderPath string) ([]DriveItem, error) {
	c.logger.Debug("Recursively listing PDF files in: %s", folderPath)

	var pdfFiles []DriveItem
	if err := c.listFilesRecursiveHelper(ctx, folderPath, &pdfFiles); err != nil {
		return nil, err
	}

	c.logger.Info("Found %d PDF files in OneDrive", len(pdfFiles))
	return pdfFiles, nil
}

// listFilesRecursiveHelper is a helper function for recursive listing
func (c *Client) listFilesRecursiveHelper(ctx context.Context, folderPath string, pdfFiles *[]DriveItem) error {
	items, err := c.ListFiles(ctx, folderPath)
	if err != nil {
		return err
	}

	for _, item := range items {
		// If it's a folder, recurse into it
		if item.Folder != nil {
			subPath := path.Join(folderPath, item.Name)
			if err := c.listFilesRecursiveHelper(ctx, subPath, pdfFiles); err != nil {
				c.logger.Debug("Warning: failed to list folder %s: %v", subPath, err)
				continue
			}
		} else if item.File != nil {
			// If it's a PDF file, add it to the list
			if strings.HasSuffix(strings.ToLower(item.Name), ".pdf") {
				*pdfFiles = append(*pdfFiles, item)
			}
		}
	}

	return nil
}

// DownloadFile downloads a file from OneDrive
func (c *Client) DownloadFile(ctx context.Context, item DriveItem, destPath string) error {
	c.logger.Debug("Downloading file: %s", item.Name)

	// Get download URL
	downloadURL := item.DownloadURL
	if downloadURL == "" {
		// If download URL is not in the item, fetch it
		var err error
		downloadURL, err = c.getDownloadURL(ctx, item.ID)
		if err != nil {
			return fmt.Errorf("failed to get download URL: %w", err)
		}
	}

	// Download the file
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create destination file
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Ensure parent directory exists
	if err := ensureDir(destPath); err != nil {
		return err
	}

	if err := writeFile(destPath, out); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	c.logger.Debug("Downloaded: %s (%d bytes)", item.Name, len(out))
	return nil
}

// getDownloadURL gets the download URL for a file
func (c *Client) getDownloadURL(ctx context.Context, itemID string) (string, error) {
	token, err := c.auth.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/me/drive/items/%s", graphAPIBaseURL, itemID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get item (status %d): %s", resp.StatusCode, string(body))
	}

	var item DriveItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return "", err
	}

	return item.DownloadURL, nil
}

// buildListURL builds the URL for listing files in a folder
func (c *Client) buildListURL(folderPath string) string {
	// Clean the path
	folderPath = strings.Trim(folderPath, "/")

	if folderPath == "" || folderPath == "." {
		// List root folder
		return fmt.Sprintf("%s/me/drive/root/children", graphAPIBaseURL)
	}

	// List specific folder by path
	encodedPath := url.PathEscape(folderPath)
	return fmt.Sprintf("%s/me/drive/root:/%s:/children", graphAPIBaseURL, encodedPath)
}

// Helper functions

func ensureDir(filePath string) error {
	dir := path.Dir(filePath)
	return mkdirAll(dir, 0755)
}

// Abstracted for testing
var mkdirAll = func(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

var writeFile = func(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

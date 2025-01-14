package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/version"
)

const (
	versionCheckURL  = "https://notesankify.com/version.json"
	githubVersionURL = "https://api.github.com/repos/kpauljoseph/notesankify/releases/latest"
	userAgent        = "NotesAnkify-Updater"
	checkInterval    = 24 * time.Hour
)

type Checker struct {
	client      *http.Client
	logger      *logger.Logger
	lastChecked time.Time
}

func NewChecker(logger *logger.Logger) *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (c *Checker) CheckForUpdates() (*UpdateInfo, error) {
	// Rate limit checks
	if time.Since(c.lastChecked) < time.Hour {
		return nil, nil
	}
	c.lastChecked = time.Now()

	c.logger.Debug("Checking for updates...")

	// Try primary endpoint first
	info, err := c.checkPrimaryEndpoint()
	if err != nil {
		c.logger.Debug("Primary endpoint failed, falling back to GitHub: %v", err)
		return c.checkGitHubAPI()
	}
	return info, nil
}

func (c *Checker) checkPrimaryEndpoint() (*UpdateInfo, error) {
	resp, err := c.client.Get(versionCheckURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var versionInfo VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("failed to decode version info: %w", err)
	}

	currentVersion := strings.TrimPrefix(version.Version, "v")
	latestVersion := strings.TrimPrefix(versionInfo.LatestVersion, "v")

	platformKey := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "darwin" {
		platformKey = "darwin/all"
	}

	downloadURL, ok := versionInfo.PlatformDownloads[platformKey]
	if !ok {
		return nil, fmt.Errorf("no download available for platform %s", platformKey)
	}

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		UpdateMessage:  versionInfo.UpdateMessage,
		DownloadURL:    downloadURL,
		IsAvailable:    compareVersions(currentVersion, latestVersion) < 0,
		ForceUpdate:    versionInfo.ForceUpdate,
	}, nil
}

func (c *Checker) checkGitHubAPI() (*UpdateInfo, error) {
	req, err := http.NewRequest("GET", githubVersionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub release: %w", err)
	}

	currentVersion := strings.TrimPrefix(version.Version, "v")
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		UpdateMessage:  release.Body,
		DownloadURL:    release.HTMLURL,
		IsAvailable:    compareVersions(currentVersion, latestVersion) < 0,
		ForceUpdate:    false,
	}, nil
}

// compareVersions returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}

	if len(parts1) < len(parts2) {
		return -1
	}
	if len(parts1) > len(parts2) {
		return 1
	}
	return 0
}

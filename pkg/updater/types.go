package updater

type VersionResponse struct {
	LatestVersion     string            `json:"latest_version"`
	MinVersion        string            `json:"min_version"`
	DownloadURL       string            `json:"download_url"`
	ReleaseNotesURL   string            `json:"release_notes_url"`
	UpdateMessage     string            `json:"update_message"`
	ForceUpdate       bool              `json:"force_update"`
	PlatformDownloads map[string]string `json:"platform_downloads"`
}

type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	UpdateMessage  string
	DownloadURL    string
	IsAvailable    bool
	ForceUpdate    bool
}

type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

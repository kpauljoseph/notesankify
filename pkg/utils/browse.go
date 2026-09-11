package utils

import (
	"os"
	"path/filepath"
)

// BrowseStartDir returns the directory the folder picker should open in.
//
// It prefers the directory already chosen in the UI. Otherwise it returns an
// app-owned folder directly under the user's home directory, creating it if
// needed. The picker must not start in the home directory itself: Fyne's
// dialog probes every entry it lists, and on macOS that triggers a separate
// privacy prompt for Desktop, Documents, Downloads, Photos, Music and network
// volumes before the dialog appears.
func BrowseStartDir(current string) (string, error) {
	if current != "" {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			return current, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(home, DefaultBrowseDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

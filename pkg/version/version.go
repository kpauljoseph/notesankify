package version

var (
	Version   = "0.1.0"
	CommitSHA = "unknown"
	BuildTime = "unknown"
)

func GetVersionInfo() string {
	return "NotesAnkify " + Version
}

func GetDetailedVersionInfo() string {
	return "NotesAnkify\n" +
		"Version:  " + Version + "\n" +
		"Commit:   " + CommitSHA + "\n" +
		"Built:    " + BuildTime
}

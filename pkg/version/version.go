package version

// Version information, populated via generated.go during build
var (
	Version   = "dev"
	CommitSHA = "unknown"
	BuildTime = "unknown"
)

func GetVersionInfo() string {
	return Version
}

func GetDetailedVersionInfo() string {
	return "NotesAnkify\n" +
		"Version:  " + Version + "\n" +
		"Commit:   " + CommitSHA + "\n" +
		"Built:    " + BuildTime
}

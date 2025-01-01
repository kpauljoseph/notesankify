package version

var (
	// These values are injected during build - DO NOT MODIFY
	Version   = "VERSION_PLACEHOLDER"
	CommitSHA = "COMMIT_PLACEHOLDER"
)

func GetVersionInfo() string {
	return "NotesAnkify " + Version
}

func GetDetailedVersionInfo() string {
	return "NotesAnkify\n" +
		"Version:  " + Version + "\n" +
		"Commit:   " + CommitSHA + "\n"
}

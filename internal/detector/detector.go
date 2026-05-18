package detector

import "context"

// ProcessDetector detects which .mscz files are currently open in MuseScore.
type ProcessDetector interface {
	// OpenFiles returns the list of .mscz paths currently open in MuseScore.
	OpenFiles() ([]string, error)
	// Watch sends on the returned channel whenever the set of open files changes.
	// The channel is closed when ctx is cancelled.
	Watch(ctx context.Context) (<-chan []string, error)
}

package activities

import (
	"context"
	"os"
)

// CreateWorkingDirectory a Temporal Activity that creates a temporary working directory
// and returns the path to it.
func CreateWorkingDirectory(ctx context.Context) (string, error) {
	return os.MkdirTemp(os.TempDir(), "process-collect")
}

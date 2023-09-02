package core

import (
	"os"
)

func WithTempWorkspace[T interface{}](block func(srcFile string, destDir string) (T, error)) (result T, err error) {
	destDir, err := os.MkdirTemp("", "mime-processing-workspace-*")
	if err != nil {
		return result, err
	}
	defer CleanTemp(destDir)

	srcFile, err := os.CreateTemp(destDir, "source-*")
	if err != nil {
		return result, err
	} else if err = srcFile.Close(); err != nil {
		return result, err
	}

	return block(srcFile.Name(), destDir)
}

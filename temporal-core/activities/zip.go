package activities

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
)

type ZipInput struct {
	Directory       string
	DestinationPath string
}

type ZipOutput struct {
	Path string
}

func Zip(_ context.Context, input ZipInput) (ZipOutput, error) {
	file, err := os.Create(input.DestinationPath)
	if err != nil {
		return ZipOutput{}, err
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	err = filepath.Walk(input.Directory, zippingWalker(w, input.Directory))
	if err != nil {
		return ZipOutput{}, err
	}

	return ZipOutput{input.DestinationPath}, nil
}

func zippingWalker(zipWriter *zip.Writer, directory string) func(string, os.FileInfo, error) error {
	return func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		filePath, err = filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}
		zipFile, err := zipWriter.Create(filePath)
		if err != nil {
			return err
		}

		_, err = io.Copy(zipFile, file)
		if err != nil {
			return err
		}

		return nil
	}
}

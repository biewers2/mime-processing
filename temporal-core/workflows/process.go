package workflows

import (
	"fmt"
	opt "github.com/repeale/fp-go/option"
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"mime-processing-api/temporal-core/activities"
	"time"
)

type ProcessInput struct {
	S3Uri       string
	ContentType opt.Option[string]
}

type ProcessOutput struct {
	S3ArchiveUri string
}

func Process(ctx workflow.Context, input ProcessInput) (ProcessOutput, error) {
	result, err := core.WithTempWorkspace(processFile(ctx, &input))
	if err != nil {
		return ProcessOutput{}, err
	}

	return ProcessOutput{result}, nil
}

func processFile(ctx workflow.Context, input *ProcessInput) func(srcFile, destDir string) (string, error) {
	return func(srcFile, destDir string) (string, error) {
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: time.Second * 30,
		})

		// Download file
		if err := download(ctx, input.S3Uri, srcFile); err != nil {
			return "", err
		}

		// Get MIME type
		mtype, err := mimeType(ctx, srcFile, input.ContentType)
		if err != nil {
			return "", err
		}

		// Process based on MIME type
		files, err := processMime(ctx, srcFile, destDir, mtype)
		if err != nil {
			return "", err
		}

		// Zip processed files together

		// Upload archive

		fmt.Println("{}, {}", mtype, len(files))
		return "", nil
	}
}

func download(ctx workflow.Context, s3uri, destFile string) error {
	input := activities.DownloadInput{
		S3Uri:           s3uri,
		DestinationFile: destFile,
	}

	err := workflow.ExecuteActivity(ctx, activities.Download, input).Get(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func mimeType(ctx workflow.Context, srcFile string, contentType opt.Option[string]) (string, error) {
	var mt string
	if opt.IsSome(contentType) {
		mt = contentType.Value
	} else {
		if m, err := identify(ctx, srcFile); err == nil {
			mt = m
		} else {
			return "", nil
		}
	}
	return mt, nil
}

func identify(ctx workflow.Context, srcFile string) (string, error) {
	var input = IdentifyInput{SourceFile: srcFile}
	var output IdentifyOutput

	err := workflow.ExecuteChildWorkflow(ctx, Identify, input).Get(ctx, &output)
	if err != nil {
		return "", err
	}
	return output.MimeType, nil
}

func processMime(ctx workflow.Context, srcFile, destDir, mimeType string) ([]string, error) {
	var input = ProcessMimeInput{
		SourceFile:     srcFile,
		DestinationDir: destDir,
		MimeType:       mimeType,
	}
	var output ProcessMimeOutput

	err := workflow.ExecuteChildWorkflow(ctx, ProcessMime, input).Get(ctx, &output)
	if err != nil {
		return nil, err
	}
	return output.Files, nil
}

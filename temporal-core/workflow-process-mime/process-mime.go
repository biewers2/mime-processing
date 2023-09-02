package workflow_process_mime

import (
	"fmt"
	"go.temporal.io/sdk/workflow"
	"path"
	"time"
)

type ProcessMimeInput struct {
	SourceS3Uri string `json:"source_s3_uri"`
	OutputS3Uri string `json:"output_s3_uri"`
	MimeType    string `json:"mimetype"`
}

type ProcessMimeOutput struct {
	EmbeddedCount int              `json:"embedded_count"`
	FailureCount  int              `json:"failure_count"`
	Embedded      []EmbeddedS3Info `json:"embedded"`
	Failures      []FailureInfo    `json:"failures"`
}

func ProcessMime(ctx workflow.Context, input ProcessMimeInput) (ProcessMimeOutput, error) {
	// Configure Options
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 2,
		TaskQueue:           "rusty-mime-processing",
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	sessCtx := ctx

	// Set up session
	//sessOpts := &workflow.SessionOptions{
	//	CreationTimeout:  time.Minute * 2,
	//	ExecutionTimeout: time.Minute * 2,
	//}
	//sessCtx, err := workflow.CreateSession(ctx, sessOpts)
	//if err != nil {
	//	return ProcessMimeOutput{}, err
	//}
	//defer workflow.CompleteSession(sessCtx)

	// Create workspace
	sourcePath, outputDir, err := createRustyWorkspace(sessCtx)
	if err != nil {
		return ProcessMimeOutput{}, err
	}

	// Download file
	_, err = downloadRustyFile(sessCtx, input.SourceS3Uri, sourcePath)
	if err != nil {
		return ProcessMimeOutput{}, err
	}

	// Process then upload files concurrently
	procInput := ProcessRustyFileInput{
		SourcePath: sourcePath,
		OutputDir:  outputDir,
		Mimetype:   input.MimeType,
	}
	procOutput, err := processRustyFile(sessCtx, procInput, input.OutputS3Uri)
	if err != nil {
		return ProcessMimeOutput{}, err
	}

	// Destroy workspace
	err = destroyRustyWorkspace(sessCtx, sourcePath, outputDir)
	if err != nil {
		return ProcessMimeOutput{}, err
	}

	return procOutput, nil
}

func createRustyWorkspace(ctx workflow.Context) (string, string, error) {
	var output CreateRustyWorkspaceOutput
	err := workflow.ExecuteActivity(ctx, "create_rusty_workspace", CreateRustyWorkspaceInput{}).Get(ctx, &output)
	if err != nil {
		return "", "", err
	}

	return output.SourcePath, output.OutputDir, nil
}

func destroyRustyWorkspace(ctx workflow.Context, sourcePath, outputDir string) error {
	input := DestroyRustyWorkspaceInput{
		SourcePath: sourcePath,
		OutputDir:  outputDir,
	}

	var output DestroyRustyWorkspaceOutput
	err := workflow.ExecuteActivity(ctx, "destroy_rusty_workspace", input).Get(ctx, &output)
	if err != nil {
		return err
	}
	return nil
}

func downloadRustyFile(ctx workflow.Context, sourceS3Uri, outputPath string) (int64, error) {
	input := DownloadRustyFileInput{
		SourceS3Uri: sourceS3Uri,
		OutputPath:  outputPath,
	}

	var output DownloadRustyFileOutput
	err := workflow.ExecuteActivity(ctx, "download_rusty_file", input).Get(ctx, &output)
	if err != nil {
		return 0, err
	}
	return output.Bytes, err
}

func processRustyFile(ctx workflow.Context, input ProcessRustyFileInput, outputS3Uri string) (ProcessMimeOutput, error) {
	var output ProcessRustyFileOutput
	err := workflow.ExecuteActivity(ctx, "process_rusty_file", input).Get(ctx, &output)
	if err != nil {
		return ProcessMimeOutput{}, err
	}

	var uploads []workflow.Future
	var embeddedS3 []EmbeddedS3Info

	for _, p := range output.Processed {
		println("Processed:", p.Path)
		s3Uri := fmt.Sprintf("%s/%s/%s", outputS3Uri, p.DupeId, path.Base(p.Path))
		println("S3:", s3Uri)
		uploads = append(uploads, uploadRustyFile(ctx, p.Path, s3Uri))
	}

	for _, e := range output.Embedded {
		s3Uri := fmt.Sprintf("%s/%s/%s", outputS3Uri, e.DupeId, path.Base(e.Path))
		uploads = append(uploads, uploadRustyFile(ctx, e.Path, s3Uri))
		embeddedS3 = append(embeddedS3, EmbeddedS3Info{
			EmbeddedS3Uri: s3Uri,
			MimeType:      e.MimeType,
			DupeId:        e.DupeId,
		})
	}

	for _, u := range uploads {
		_ = u.Get(ctx, nil)
	}

	return ProcessMimeOutput{
		EmbeddedCount: len(embeddedS3),
		FailureCount:  len(output.Failures),
		Embedded:      embeddedS3,
		Failures:      output.Failures,
	}, nil
}

func uploadRustyFile(ctx workflow.Context, sourceFilePath, outputS3Uri string) workflow.Future {
	input := UploadRustyFileInput{
		SourceFilePath: sourceFilePath,
		OutputS3Uri:    outputS3Uri,
	}

	return workflow.ExecuteActivity(ctx, "upload_rusty_file", input)
}

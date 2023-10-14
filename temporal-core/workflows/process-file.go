package workflows

import (
	"fmt"
	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"path"
	"time"
)

type ProcessFileInput struct {
	S3Uri       string
	MimeType    string
	CollectWfId string
}

type ProcessFileOutput struct {
	S3OutputDir string
}

type ProcessRustyFileInput struct {
	SourceS3Uri    string `json:"source_s3_uri"`
	OutputDirS3Uri string `json:"output_dir_s3_uri"`
	Mimetype       string `json:"mimetype"`
}

type ProcessRustyFileOutput struct {
	OriginalS3Uri  string     `json:"original_s3_uri"`
	ProcessedFiles []FileInfo `json:"processed_files"`
	EmbeddedFiles  []FileInfo `json:"embedded_files"`
}

type FileInfo struct {
	S3Uri    string `json:"s3_uri"`
	MimeType string `json:"mimetype"`
	Id       string `json:"id"`
}

func ProcessFile(ctx workflow.Context, input ProcessFileInput) (ProcessFileOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 2,
		TaskQueue:           core.RustyProcessTaskQueue,
	})

	outputS3Uri, err := processFile(ctx, input)
	if err != nil {
		return ProcessFileOutput{}, err
	}

	return ProcessFileOutput{outputS3Uri}, nil
}

func processFile(ctx workflow.Context, input ProcessFileInput) (string, error) {
	logger := workflow.GetLogger(ctx)
	totalFiles := 0

	outputS3Uri, err := s3OutputDirFromSource(input.S3Uri)
	if err != nil {
		return "", err
	}

	rootPrfInput := ProcessRustyFileInput{
		SourceS3Uri:    input.S3Uri,
		OutputDirS3Uri: outputS3Uri,
		Mimetype:       input.MimeType,
	}
	inputsToProcess := []ProcessRustyFileInput{rootPrfInput}

	for {
		var aes []workflow.Future

		// Start workflows in parallel
		for _, input := range inputsToProcess {
			logger.Info("Starting workflow", "input", input)
			aes = append(aes, workflow.ExecuteActivity(ctx, "process_rusty_file", input))
			if err != nil {
				return "", err
			}
		}
		inputsToProcess = []ProcessRustyFileInput{}

		// Collect files from each activity
		for _, ae := range aes {
			var prfOutput ProcessRustyFileOutput
			err := ae.Get(ctx, &prfOutput)
			if err != nil {
				logger.Error("Error getting workflow result", err)
			}
			// Signal to the collect workflow to add the original processed file, if it's not the root file
			if prfOutput.OriginalS3Uri != input.S3Uri {
				err = signalCollectToAdd(ctx, prfOutput.OriginalS3Uri, input.CollectWfId)
				if err != nil {
					logger.Error("Error signaling collect to add", err)
				} else {
					totalFiles += 1
				}
			}

			// Signal to the collect workflow to add each processed file (text, metadata, pdf)
			for _, file := range prfOutput.ProcessedFiles {
				err = signalCollectToAdd(ctx, file.S3Uri, input.CollectWfId)
				if err != nil {
					logger.Error("Error signaling collect to add", err)
				} else {
					totalFiles += 1
				}
			}

			// Add embedded files to the inputs to be processed
			for _, file := range prfOutput.EmbeddedFiles {
				outputS3Uri, _ := path.Split(file.S3Uri)
				inputsToProcess = append(inputsToProcess, ProcessRustyFileInput{
					SourceS3Uri:    file.S3Uri,
					OutputDirS3Uri: outputS3Uri,
					Mimetype:       file.MimeType,
				})
			}
		}

		// Break if no more embedded files to processFile
		if len(inputsToProcess) == 0 {
			break
		}
	}

	logger.Info("Signaling collect to finish", "totalFiles", totalFiles)
	err = signalCollectToFinish(ctx, input.CollectWfId, totalFiles)
	if err != nil {
		return "", err
	}

	return outputS3Uri, nil
}

func signalCollectToAdd(ctx workflow.Context, s3Uri, collectWfId string) error {
	addSignal := AddSignal{S3Uri: s3Uri}
	signal := workflow.SignalExternalWorkflow(ctx, collectWfId, "", "add", addSignal)
	return signal.Get(ctx, nil)
}

func signalCollectToFinish(ctx workflow.Context, collectWfId string, totalFiles int) error {
	finishSignal := FinishSignal{totalFiles}
	signal := workflow.SignalExternalWorkflow(ctx, collectWfId, "", "finish", finishSignal)
	return signal.Get(ctx, nil)
}

func s3OutputDirFromSource(sourceS3Uri string) (string, error) {
	bucket, key, err := core.ParseS3Uri(sourceS3Uri)
	if err != nil {
		return "", err
	}

	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("s3://%s/%s-%s)", bucket, key, id.String()), nil
}

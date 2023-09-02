package workflows

import (
	"fmt"
	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"mime-processing-api/temporal-core/workflow-process-mime"
	"time"
)

type ProcessInput struct {
	S3Uri    string
	MimeType string
}

type ProcessOutput struct {
	S3ArchiveUri string
}

func Process(ctx workflow.Context, input ProcessInput) (ProcessOutput, error) {
	result, err := process(ctx, &input)
	if err != nil {
		return ProcessOutput{}, err
	}

	return ProcessOutput{result}, nil
}

func process(ctx workflow.Context, input *ProcessInput) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 30,
	})

	// Identify MIME type
	var mimetype string
	if input.MimeType == "" {
		mtype, err := identify(ctx, input.S3Uri)
		if err != nil {
			return "", err
		}
		mimetype = mtype
	} else {
		mimetype = input.MimeType
	}

	// Process based on MIME type
	err := processMime(ctx, input.S3Uri, mimetype)
	if err != nil {
		return "", err
	}

	// Zip processed files together

	// Upload archive

	return "", nil
}

func identify(ctx workflow.Context, s3Uri string) (string, error) {
	var input = IdentifyInput{S3Uri: s3Uri}
	var output IdentifyOutput

	err := workflow.ExecuteChildWorkflow(ctx, Identify, input).Get(ctx, &output)
	if err != nil {
		return "", err
	}

	return output.MimeType, nil
}

func processMime(ctx workflow.Context, s3Uri, mimeType string) error {
	outputUri, err := outputS3Uri(s3Uri)
	if err != nil {
		return err
	}

	mimesToProcess := []workflow_process_mime.ProcessMimeInput{
		{
			SourceS3Uri: s3Uri,
			OutputS3Uri: outputUri,
			MimeType:    mimeType,
		},
	}

	for {
		var wes []workflow.ChildWorkflowFuture

		// Start workflows in parallel
		for _, mime := range mimesToProcess {
			wes = append(wes, workflow.ExecuteChildWorkflow(ctx, workflow_process_mime.ProcessMime, mime))
		}
		mimesToProcess = []workflow_process_mime.ProcessMimeInput{}

		// Collect embedded files from each workflow
		for _, we := range wes {
			var output workflow_process_mime.ProcessMimeOutput
			err := we.Get(ctx, &output)
			if err != nil {
				return err
			}

			for _, e := range output.Embedded {
				mimesToProcess = append(mimesToProcess, workflow_process_mime.ProcessMimeInput{
					SourceS3Uri: e.EmbeddedS3Uri,
					OutputS3Uri: outputUri,
					MimeType:    e.MimeType,
				})
			}
		}

		// Break if no more embedded files to process
		if len(mimesToProcess) == 0 {
			break
		}
	}

	return nil
}

func outputS3Uri(sourceS3Uri string) (string, error) {
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

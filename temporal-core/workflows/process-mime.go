package workflows

import (
	"go.temporal.io/sdk/workflow"
)

type ProcessMimeInput struct {
	SourceFile     string
	DestinationDir string
	MimeType       string
}

type ProcessMimeOutput struct {
	Files []string
}

func ProcessMime(ctx workflow.Context, input ProcessMimeInput) (ProcessMimeOutput, error) {
	//logger := workflow.GetLogger(ctx)

	return ProcessMimeOutput{[]string{""}}, nil
}

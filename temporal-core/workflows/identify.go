package workflows

import (
	"go.temporal.io/sdk/workflow"
)

type IdentifyInput struct {
	SourceFile string
}

type IdentifyOutput struct {
	MimeType string
}

func Identify(ctx workflow.Context, input IdentifyInput) (IdentifyOutput, error) {
	//logger := activity.GetLogger(ctx)

	return IdentifyOutput{"application/mbox"}, nil
}

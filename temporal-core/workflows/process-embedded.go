package workflows

import (
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"path"
	"time"
)

type ProcessEmbeddedInput struct {
	// Reference to the original input to the parent Process workflow.
	//
	// This is used to track context if workflow needs to continue as new when it exceeds the Temporal history limit.
	RootInput ProcessInput

	// From the parent Process workflow, contains context on the workspace that this workflow is operating on.
	Workspace Workspace

	// From the parent Process workflow, contains the name of the Redis stream that activities are writing outputs to.
	StreamName string

	// Tracks the total number of files that have been processed.
	TotalCount int

	// The workflow ID of the corresponding `ForwardOutput` workflow.
	//
	// This is used to signal the workflow to terminate when all files have been processed.
	ForwardingWorkflowId string
}

// ProcessEmbedded is a workflow that is executed by the `Process` workflow.
//
// This workflow is responsible for receiving outputs from the output stream defined by the `StreamName` field in the
// `ProcessEmbeddedInput` struct. It then starts a `ProcessRustyFile` workflow for each entry in the output stream.
func ProcessEmbedded(ctx workflow.Context, input ProcessEmbeddedInput) error {
	logger := workflow.GetLogger(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue: input.Workspace.StickyTaskQueue,
	})

	// Tracks the number of ProcessRustyFile activities that are still running
	prfCount := 0
	// For observability, tracks the total number of files that have been processed
	totalCount := input.TotalCount

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(
		workflow.GetSignalChannel(ctx, core.OutputSignalChannelName),
		func(c workflow.ReceiveChannel, more bool) {
			var entries []OutputStreamEntry
			c.Receive(ctx, &entries)
			prfCount += len(entries)
			totalCount += len(entries)

			for _, entry := range entries {
				parent, _ := path.Split(entry.Path)
				selector.AddFuture(
					startProcessingRustyFile(ctx, ProcessRustyFileInput{
						Path:             entry.Path,
						Directory:        parent,
						Mimetype:         entry.Mimetype,
						Types:            input.RootInput.Types,
						OutputStreamName: input.StreamName,
					}),
					func(f workflow.Future) {
						err := f.Get(ctx, nil)
						prfCount--
						if err != nil {
							return
						}
					},
				)
			}
		},
	)

	rootPrfEnded := false
	for {
		if !rootPrfEnded {
			var c int
			if workflow.GetSignalChannel(ctx, core.TerminateSignalChannelName).ReceiveAsync(&c) {
				rootPrfEnded = true
			}
		}
		if prfCount == 0 && rootPrfEnded {
			break
		}

		if workflow.GetInfo(ctx).GetCurrentHistoryLength() > core.MaxWorkflowHistoryLength {
			selector.AddDefault(func() {})
			for selector.HasPending() {
				selector.Select(ctx)
			}
			input.TotalCount = totalCount
			err := workflow.NewContinueAsNewError(ctx, ProcessEmbedded, input)
			if err != nil {
				return err
			}
		}

		selector.Select(ctx)
	}

	return nil
}

func startProcessingRustyFile(ctx workflow.Context, input ProcessRustyFileInput) workflow.Future {
	opts := workflow.GetActivityOptions(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 3, // TODO - update based on file size?/file type?
		TaskQueue:           opts.TaskQueue,
		RetryPolicy:         &core.DefaultRetryPolicy,
	})
	return workflow.ExecuteActivity(ctx, "ProcessRustyFile", input)
}

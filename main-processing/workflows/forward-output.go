package workflows

import (
	"core"
	"go.temporal.io/sdk/workflow"
	"time"
)

type ForwardOutputInput struct {
	StreamName        string
	ProcessWorkflowId string
	LastOutputId      string
}

func ForwardOutput(ctx workflow.Context, input ForwardOutputInput) error {
	ctx, cancel := workflow.WithCancel(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Second * 30,
		StartToCloseTimeout:    time.Second * 10,
		TaskQueue:              core.TaskQueue,
		RetryPolicy:            &core.DefaultRetryPolicy,
	})

	lastId := input.LastOutputId
	terminated := false

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(
		workflow.GetSignalChannel(ctx, core.TerminateSignalChannelName),
		func(c workflow.ReceiveChannel, more bool) {
			terminated = true
			cancel()
		},
	)

	callback := signalWorkflowCallback(ctx, input.ProcessWorkflowId, &lastId)
	selector.AddFuture(executeQuery(ctx, input.StreamName, lastId), callback)
	for !terminated {
		if workflow.GetInfo(ctx).GetCurrentHistoryLength() > core.MaxWorkflowHistoryLength {
			for selector.HasPending() {
				selector.Select(ctx)
			}
			input.LastOutputId = lastId
			err := workflow.NewContinueAsNewError(ctx, ForwardOutput, input)
			if err != nil {
				return err
			}
		}

		selector.Select(ctx)
		selector.AddFuture(executeQuery(ctx, input.StreamName, lastId), callback)
	}

	return nil
}

func executeQuery(ctx workflow.Context, stream, id string) workflow.Future {
	return workflow.ExecuteActivity(ctx, QueryOutputStream, QueryOutputStreamInput{
		StreamName: stream,
		StartId:    id,
	})
}

func signalWorkflowCallback(ctx workflow.Context, workflowId string, lastStreamOutputId *string) func(workflow.Future) {
	return func(f workflow.Future) {
		var output QueryOutputStreamOutput
		err := f.Get(ctx, &output)
		if err != nil {
			return
		}
		if n := len(output.Entries); n > 0 {
			*lastStreamOutputId = output.Entries[n-1].Id
		}

		workflow.SignalExternalWorkflow(
			ctx,
			workflowId,
			"",
			core.OutputSignalChannelName,
			output.Entries,
		)
	}
}

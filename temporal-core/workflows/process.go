package workflows

import (
	"go.temporal.io/sdk/workflow"
)

type ProcessInput struct {
	InputS3Uri  string
	OutputS3Uri string
	MimeType    string
}

type ProcessOutput struct {
	OutputS3Uri string
}

func Process(ctx workflow.Context, input ProcessInput) (ProcessOutput, error) {
	logger := workflow.GetLogger(ctx)

	collectWfId := workflowId(ctx, "collect")
	ctx = workflow.WithWorkflowID(ctx, collectWfId)
	collectInput := CollectInput{
		OutputS3Uri: input.OutputS3Uri,
	}
	collectWE := workflow.ExecuteChildWorkflow(ctx, Collect, collectInput)

	ctx = workflow.WithWorkflowID(ctx, workflowId(ctx, "processFile"))
	processFileInput := ProcessFileInput{
		S3Uri:       input.InputS3Uri,
		MimeType:    input.MimeType,
		CollectWfId: collectWfId,
	}
	processFileWE := workflow.ExecuteChildWorkflow(ctx, ProcessFile, processFileInput)

	var processOutput ProcessFileOutput
	err := processFileWE.Get(ctx, &processOutput)
	if err != nil {
		logger.Error("Unable to get ProcessFile WF result:", err)
		return ProcessOutput{}, err
	}
	// Clean up S3 files located in processOutput.S3OutputDir

	var collectOutput CollectOutput
	err = collectWE.Get(ctx, &collectOutput)
	if err != nil {
		logger.Error("Unable to get Collect WF result:", err)
		return ProcessOutput{}, err
	}

	return ProcessOutput{input.OutputS3Uri}, nil
}

func workflowId(ctx workflow.Context, name string) string {
	return workflow.GetInfo(ctx).WorkflowExecution.ID + "-" + name
}

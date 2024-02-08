package workflows

import (
	"fmt"
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"time"
)

type ProcessInput struct {
	InputS3Uri  string   `json:"input_s3_uri"`
	OutputS3Uri string   `json:"output_s3_uri"`
	MimeType    string   `json:"mimetype"`
	Types       []string `json:"types"`
	Recurse     bool     `json:"recurse"`
}

type CreateWorkspaceInput struct{}

type Workspace struct {
	RootPath        string `json:"root_path"`
	Directory       string `json:"directory"`
	StickyTaskQueue string `json:"sticky_task_queue"`
}

type CreateWorkspaceOutput = Workspace

type RemoveWorkspaceInput struct {
	Paths []string `json:"paths"`
}

type DownloadInput struct {
	S3Uri string `json:"s3_uri"`
	Path  string `json:"path"`
}

type UploadInput struct {
	Path  string `json:"path"`
	S3Uri string `json:"s3_uri"`
}

type ZipInput struct {
	Directory string `json:"directory"`
}

type ZipOutput struct {
	Path string `json:"path"`
}

type ProcessRustyFileInput struct {
	Path             string   `json:"path"`
	Directory        string   `json:"directory"`
	Mimetype         string   `json:"mimetype"`
	Types            []string `json:"types"`
	OutputStreamName string   `json:"output_stream_name"`
}

func Process(ctx workflow.Context, input ProcessInput) error {
	workspace, err := createWorkspace(ctx)
	if err != nil {
		return err
	}

	// Bind all activities to the machine that created the workspace via a "sticky" task queue
	actOpts := workflow.GetActivityOptions(ctx)
	actOpts.TaskQueue = workspace.StickyTaskQueue
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	err = download(ctx, input.InputS3Uri, workspace.RootPath)
	if err != nil {
		return err
	}

	streamName := workflow.GetInfo(ctx).OriginalRunID
	rootPrfTask := startProcessingRustyFile(ctx, ProcessRustyFileInput{
		Path:             workspace.RootPath,
		Directory:        workspace.Directory,
		Mimetype:         input.MimeType,
		Types:            input.Types,
		OutputStreamName: streamName,
	})

	if input.Recurse {
		runId := workflow.GetInfo(ctx).WorkflowExecution.RunID
		processEmbWfId := fmt.Sprintf("process-embedded-%s", runId)
		forwardingWfId := fmt.Sprintf("forward-output-%s", runId)

		processEmbCtx := workflow.WithWorkflowID(ctx, processEmbWfId)
		processEmbWf := workflow.ExecuteChildWorkflow(processEmbCtx, ProcessEmbedded, ProcessEmbeddedInput{
			RootInput:            input,
			Workspace:            workspace,
			StreamName:           streamName,
			TotalCount:           1, // Include root PRF task in total count
			ForwardingWorkflowId: forwardingWfId,
		})

		forwardingCtx := workflow.WithWorkflowID(ctx, forwardingWfId)
		forwardingWf := workflow.ExecuteChildWorkflow(forwardingCtx, ForwardOutput, ForwardOutputInput{
			StreamName:        streamName,
			ProcessWorkflowId: processEmbWfId,
			LastOutputId:      "0",
		})

		err = rootPrfTask.Get(ctx, nil)
		if err != nil {
			return err
		}
		processEmbWf.SignalChildWorkflow(ctx, core.TerminateSignalChannelName, nil)

		err = processEmbWf.Get(ctx, nil)
		if err != nil {
			return err
		}
		forwardingWf.SignalChildWorkflow(ctx, core.TerminateSignalChannelName, nil)

		err = forwardingWf.Get(ctx, nil)
		if err != nil {
			return err
		}
	} else {
		err = rootPrfTask.Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	zipPath, err := zip(ctx, workspace.Directory)
	if err != nil {
		return err
	}

	err = upload(ctx, zipPath, input.OutputS3Uri)
	if err != nil {
		return err
	}

	return removeWorkspace(ctx, []string{
		workspace.RootPath,
		workspace.Directory,
		zipPath,
	})
}

func createWorkspace(ctx workflow.Context) (Workspace, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
		TaskQueue:           core.RustyProcessTaskQueue,
		RetryPolicy:         &core.DefaultRetryPolicy,
	})

	input := CreateWorkspaceInput{}
	var workspace Workspace
	err := workflow.ExecuteActivity(ctx, "CreateWorkspace", input).Get(ctx, &workspace)

	return workspace, err
}

func download(ctx workflow.Context, s3Uri, path string) error {
	opts := workflow.GetActivityOptions(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 2, // TODO - update based on file size
		TaskQueue:           opts.TaskQueue,
		RetryPolicy:         &core.DefaultRetryPolicy,
	})

	input := DownloadInput{
		S3Uri: s3Uri,
		Path:  path,
	}
	return workflow.ExecuteActivity(ctx, "Download", input).Get(ctx, nil)
}

func zip(ctx workflow.Context, directory string) (string, error) {
	opts := workflow.GetActivityOptions(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 2, // TODO - update based on file size
		TaskQueue:           opts.TaskQueue,
		RetryPolicy:         &core.DefaultRetryPolicy,
	})

	input := ZipInput{directory}
	var output ZipOutput
	err := workflow.ExecuteActivity(ctx, "Zip", input).Get(ctx, &output)
	if err != nil {
		return "", err
	}

	return output.Path, nil
}

func upload(ctx workflow.Context, path, s3Uri string) error {
	opts := workflow.GetActivityOptions(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 2, // TODO - update based on file size
		TaskQueue:           opts.TaskQueue,
		RetryPolicy:         &core.DefaultRetryPolicy,
	})

	input := UploadInput{
		Path:  path,
		S3Uri: s3Uri,
	}
	return workflow.ExecuteActivity(ctx, "Upload", input).Get(ctx, nil)
}

func removeWorkspace(ctx workflow.Context, paths []string) error {
	opts := workflow.GetActivityOptions(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
		TaskQueue:           opts.TaskQueue,
		RetryPolicy:         &core.DefaultRetryPolicy,
	})

	input := RemoveWorkspaceInput{paths}
	return workflow.ExecuteActivity(ctx, "RemoveWorkspace", input).Get(ctx, nil)
}

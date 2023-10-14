package workflows

import (
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"mime-processing-api/temporal-core/activities"
	"os"
	"path"
)

type AddSignal struct {
	S3Uri string
}

type FinishSignal struct {
	Total int
}

type CollectInput struct {
	OutputS3Uri string
}

type CollectOutput struct {
	Count       int
	OutputS3Uri string
}

// Collect collects files sent through "add" signals, zips them, and uploads the zip file to S3.
func Collect(ctx workflow.Context, input CollectInput) (CollectOutput, error) {
	// Create a session to ensure all local file activity is synced
	//sessCtx, err := workflow.CreateSession(ctx, &workflow.SessionOptions{})
	//if err != nil {
	//	return CollectOutput{}, err
	//}
	//defer workflow.CompleteSession(sessCtx)

	sessCtx := ctx
	sessCtx = workflow.WithActivityOptions(sessCtx, core.DefaultActivityOptions)
	logger := workflow.GetLogger(sessCtx)

	// Create a working directory to place downloaded files
	workingDir, err := createWorkingDir(sessCtx)
	if err != nil {
		return CollectOutput{}, err
	}
	defer os.RemoveAll(workingDir)

	// Listen for "add" and "finish" signals
	total, downloads := listenForFiles(sessCtx, workingDir)
	logger.Debug("Finished processing files", "total", total)

	// Wait for all downloads to complete
	var lastDlPath string
	for _, dl := range downloads {
		var output activities.DownloadOutput
		err := dl.Get(sessCtx, &output)
		if err != nil {
			return CollectOutput{}, err
		}
		lastDlPath = output.Path
	}

	// If there are no files to collect, return
	if total == 0 {
		logger.Warn("No files to collect")
		return CollectOutput{}, nil
	}

	// Zip if there are multiple files, otherwise just use the last downloaded file
	var pathToUpload string
	if total > 1 {
		// Create a separate temp directory for the zip file, otherwise the zip file will be included
		// in the resulting zip file
		zipDir, err := createWorkingDir(sessCtx)
		if err != nil {
			return CollectOutput{}, err
		}
		defer os.RemoveAll(workingDir)

		zipPath := path.Join(zipDir, "collected.zip")
		pathToUpload, err = zip(sessCtx, workingDir, zipPath)
		if err != nil {
			return CollectOutput{Count: total}, err
		}
	} else {
		pathToUpload = lastDlPath
	}

	// Upload to S3
	s3Uri, err := upload(sessCtx, pathToUpload, input.OutputS3Uri)
	if err != nil {
		return CollectOutput{}, err
	}

	return CollectOutput{total, s3Uri}, nil
}

// listenForFiles listens for "add" and "finish" signals, and returns the total number of files
// to collect and a slice of futures for each download activity.
func listenForFiles(ctx workflow.Context, workingDir string) (totalFiles int, downloads []workflow.Future) {
	logger := workflow.GetLogger(ctx)

	count := 0
	finished := false
	selector := workflow.NewSelector(ctx)

	var addSignal AddSignal
	addChan := workflow.GetSignalChannel(ctx, "add")
	selector.AddReceive(addChan, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &addSignal)
		dl, err := download(ctx, addSignal.S3Uri, workingDir)
		if err != nil {
			logger.Error("Error adding file to collect", "error", err)
		} else {
			downloads = append(downloads, dl)
			count += 1
		}
	})

	var finishSignal FinishSignal
	finishChan := workflow.GetSignalChannel(ctx, "finish")
	selector.AddReceive(finishChan, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &finishSignal)
		totalFiles = finishSignal.Total
		finished = true
	})

	// Ensure all "add" signals have been received, as "Select" chooses futures randomly and could
	// select the "finish" signal before all "add" signals have been received.
	for !finished || count < totalFiles {
		selector.Select(ctx)
	}

	return totalFiles, downloads
}

func createWorkingDir(ctx workflow.Context) (string, error) {
	var dir string
	err := workflow.ExecuteActivity(ctx, activities.CreateWorkingDirectory, nil).Get(ctx, &dir)
	if err != nil {
		return "", err
	}
	return dir, nil
}

// upload uploads a file to S3 and returns the S3 URI.
func upload(ctx workflow.Context, srcPath, s3Uri string) (string, error) {
	logger := workflow.GetLogger(ctx)

	logger.Debug("Uploading", srcPath, "to", s3Uri)
	input := activities.UploadInput{
		SourcePath: srcPath,
		S3Uri:      s3Uri,
	}
	var output activities.UploadOutput
	err := workflow.ExecuteActivity(ctx, activities.Upload, input).Get(ctx, &output)
	if err != nil {
		return "", err
	}

	return output.S3Uri, nil
}

// download downloads a file from S3 and returns a future for the download activity.
func download(ctx workflow.Context, s3Uri, directory string) (workflow.Future, error) {
	logger := workflow.GetLogger(ctx)

	_, key, err := core.ParseS3Uri(s3Uri)
	if err != nil {
		return nil, err
	}

	logger.Debug("Downloading", s3Uri, "to", directory)
	input := activities.DownloadInput{
		S3Uri:           s3Uri,
		DestinationPath: path.Join(directory, key),
	}
	return workflow.ExecuteActivity(ctx, activities.Download, input), nil
}

// zip zips a directory and returns the path to the zip file.
func zip(ctx workflow.Context, directory, destPath string) (string, error) {
	logger := workflow.GetLogger(ctx)

	logger.Debug("Zipping files in", directory)
	input := activities.ZipInput{
		Directory:       directory,
		DestinationPath: destPath,
	}
	var output activities.ZipOutput
	err := workflow.ExecuteActivity(ctx, activities.Zip, input).Get(ctx, &output)
	if err != nil {
		return "", err
	}

	// zip activity from directory
	return output.Path, nil
}

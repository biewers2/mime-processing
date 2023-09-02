package workflows

import (
	"go.temporal.io/sdk/workflow"
	core "mime-processing-api/temporal-core"
	"mime-processing-api/temporal-core/activities"
	"os"
)

type IdentifyInput struct {
	S3Uri string
}

type IdentifyOutput struct {
	MimeType string
}

func Identify(ctx workflow.Context, input IdentifyInput) (IdentifyOutput, error) {
	mimetype, err := head(ctx, input.S3Uri)
	if err != nil {
		return IdentifyOutput{}, err
	}

	if mimetype == "" {
		f, err := os.CreateTemp("", "mime-processing-api")
		if err != nil {
			return IdentifyOutput{}, err
		}
		defer core.CleanTemp(f.Name())

		err = download(ctx, input.S3Uri, f.Name())
		if err != nil {
			return IdentifyOutput{}, err
		}

		// TODO | identify-mime
		mimetype = "application/mbox"
	}

	return IdentifyOutput{mimetype}, nil
}

func head(ctx workflow.Context, s3uri string) (string, error) {
	var input = activities.HeadInput{S3Uri: s3uri}
	var output activities.HeadOutput

	err := workflow.ExecuteActivity(ctx, activities.HeadObject, input).Get(ctx, &output)
	if err != nil {
		return "", err
	}

	if output.S3Header.ContentType == nil {
		return "", nil
	} else {
		return *output.S3Header.ContentType, nil
	}
}

func download(ctx workflow.Context, s3uri, destFile string) error {
	input := activities.DownloadInput{
		S3Uri:           s3uri,
		DestinationFile: destFile,
	}

	err := workflow.ExecuteActivity(ctx, activities.Download, input).Get(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

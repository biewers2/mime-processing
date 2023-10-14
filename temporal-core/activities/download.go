package activities

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.temporal.io/sdk/temporal"
	core "mime-processing-api/temporal-core"
	"os"
	"path"
)

type DownloadInput struct {
	S3Uri           string
	DestinationPath string
}

type DownloadOutput struct {
	Path  string
	Bytes int64
}

// Download downloads a file from S3 to a local file.
func Download(_ context.Context, input DownloadInput) (DownloadOutput, error) {
	bucket, key, err := core.ParseS3Uri(input.S3Uri)
	if err != nil {
		err = temporal.NewNonRetryableApplicationError("Failed to parse S3 URI", "ParseS3Uri", err)
		return DownloadOutput{}, err
	}

	err = os.MkdirAll(path.Dir(input.DestinationPath), 0775)
	if err != nil {
		return DownloadOutput{}, err
	}

	file, err := os.Create(input.DestinationPath)
	if err != nil {
		return DownloadOutput{}, err
	}

	bytes, err := core.Downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return DownloadOutput{}, err
	}

	return DownloadOutput{input.DestinationPath, bytes}, nil
}

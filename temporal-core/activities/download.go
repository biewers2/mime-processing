package activities

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	core "mime-processing-api/temporal-core"
	"os"
)

type DownloadInput struct {
	S3Uri           string
	DestinationFile string
}

type DownloadOutput struct {
	Bytes int64
}

// Temporal Activity to download a file from S3 into a destination.
// Returns number of bytes downloaded.
func Download(_ context.Context, input DownloadInput) (DownloadOutput, error) {
	bucket, key, err := core.ParseS3Uri(input.S3Uri)
	if err != nil {
		return DownloadOutput{}, err
	}

	file, err := os.OpenFile(input.DestinationFile, os.O_WRONLY, 0744)
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

	return DownloadOutput{bytes}, nil
}

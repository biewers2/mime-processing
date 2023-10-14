package activities

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	core "mime-processing-api/temporal-core"
	"os"
)

type UploadInput struct {
	SourcePath string
	S3Uri      string
}

type UploadOutput struct {
	S3Uri string
}

func Upload(_ context.Context, input UploadInput) (UploadOutput, error) {
	bucket, key, err := core.ParseS3Uri(input.S3Uri)
	if err != nil {
		return UploadOutput{}, err
	}

	file, err := os.Open(input.SourcePath)
	if err != nil {
		return UploadOutput{}, err
	}

	result, err := core.Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return UploadOutput{}, err
	}

	return UploadOutput{result.Location}, nil
}

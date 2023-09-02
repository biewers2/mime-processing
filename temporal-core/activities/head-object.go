package activities

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	core "mime-processing-api/temporal-core"
)

type HeadInput struct {
	S3Uri string
}

type HeadOutput struct {
	S3Header *s3.HeadObjectOutput
}

// HeadObject
// Temporal Activity to get the header information of a file from S3.
// Returns header information as-is from S3.
func HeadObject(ctx context.Context, input HeadInput) (HeadOutput, error) {
	bucket, key, err := core.ParseS3Uri(input.S3Uri)
	if err != nil {
		return HeadOutput{}, err
	}

	output, err := core.Svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return HeadOutput{}, err
	}

	return HeadOutput{output}, nil
}

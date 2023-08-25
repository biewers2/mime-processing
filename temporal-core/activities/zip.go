package activities

import (
	"context"
)

type ZipInput struct {
	Directory       string
	DestinationFile string
}

type ZipOutput struct {
	Result int64
}

func Zip(ctx context.Context, input ZipInput) (ZipOutput, error) {
	//logger := activity.GetLogger(ctx)

	return ZipOutput{}, nil
}

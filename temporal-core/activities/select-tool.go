package activities

import "context"

type SelectToolInput struct {
}

type SelectToolOutput struct {
}

func SelectTool(ctx context.Context, input SelectToolInput) (SelectToolOutput, error) {
	return SelectToolOutput{}, nil
}

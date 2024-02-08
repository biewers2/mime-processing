package workflows

import (
	"core"
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/activity"
)

type QueryOutputStreamInput struct {
	StreamName string `json:"stream_name"`
	StartId    string `json:"start_id"`
}

type QueryOutputStreamOutput struct {
	Entries []OutputStreamEntry `json:"entries"`
}

type OutputStreamEntry struct {
	Id       string `json:"id"`
	Path     string `json:"path"`
	Mimetype string `json:"mimetype"`
	Checksum string `json:"checksum"`
}

func QueryOutputStream(ctx context.Context, input QueryOutputStreamInput) (QueryOutputStreamOutput, error) {
	logger := activity.GetLogger(ctx)

	var results []redis.XStream
	for {
		args := redis.XReadArgs{
			Streams: []string{input.StreamName, input.StartId},
			Count:   0,
		}
		res, err := core.RedisClient.XRead(context.Background(), &args).Result()
		if errors.Is(err, redis.Nil) {
			// Record heartbeat in order for activity to be cancellable
			activity.RecordHeartbeat(ctx, nil)
			continue
		} else if err != nil {
			logger.Error("Unable to read from stream", err)
			return QueryOutputStreamOutput{}, err
		} else {
			results = res
			break
		}
	}

	var entries []OutputStreamEntry
	for _, result := range results {
		for _, msg := range result.Messages {
			entries = append(entries, OutputStreamEntry{
				Id:       msg.ID,
				Path:     msg.Values["path"].(string),
				Mimetype: msg.Values["mimetype"].(string),
				Checksum: msg.Values["checksum"].(string),
			})
		}
	}

	return QueryOutputStreamOutput{entries}, nil
}

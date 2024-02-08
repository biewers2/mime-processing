package core

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/temporal"
	"log"
	"net"
	"net/url"
	"os"
	"time"
)

type Unit = struct{}

const TaskQueue = "mime-processing"
const RustyProcessTaskQueue = "rusty-mime-processing"
const OutputSignalChannelName = "outputs"
const TerminateSignalChannelName = "terminate"
const MaxWorkflowHistoryLength = 10_000

const RedisHostKey = "REDIS_HOST"
const RedisPortKey = "REDIS_PORT"
const RedisDefaultHost = "127.0.0.1"
const RedisDefaultPort = "6379"

var DefaultRetryPolicy = temporal.RetryPolicy{
	InitialInterval:        time.Second,
	BackoffCoefficient:     2.0,
	MaximumInterval:        time.Second * 100,
	MaximumAttempts:        10,
	NonRetryableErrorTypes: []string{"S3UriParseError"},
}

var RedisClient = func() *redis.Client {
	var exists bool
	var host, port string
	if host, exists = os.LookupEnv(RedisHostKey); !exists {
		host = RedisDefaultHost
	}
	if port, exists = os.LookupEnv(RedisPortKey); !exists {
		port = RedisDefaultPort
	}

	return redis.NewClient(&redis.Options{
		Addr:     net.JoinHostPort(host, port),
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}()

var sess = func() *session.Session {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-2"),
		Credentials: credentials.NewSharedCredentials(defaults.SharedCredentialsFilename(), "default"),
	})
	if err != nil {
		log.Fatalln("Failed to initialize AWS session", err)
	}

	return sess
}()

var Svc = s3.New(sess)
var Downloader = s3manager.NewDownloader(sess)
var Uploader = s3manager.NewUploader(sess)

type S3UriParseError struct {
	S3Uri string
}

func (e *S3UriParseError) Error() string {
	return fmt.Sprintf("invalid s3 uri: '%s'", e.S3Uri)
}

func ParseS3Uri(s3uri string) (bucket string, key string, err error) {
	u, err := url.Parse(s3uri)
	if err != nil || u.Scheme != "s3" || u.Host == "" || u.Path == "" {
		return "", "", &S3UriParseError{s3uri}
	}

	path := u.Path[1:]
	return u.Host, path, nil
}

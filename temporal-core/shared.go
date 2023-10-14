package core

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"log"
	"net/url"
	"time"
)

type Unit = struct{}

const TaskQueue = "mime-processing"
const RustyProcessTaskQueue = "rusty-mime-processing"

var DefaultRetryPolicy = temporal.RetryPolicy{
	InitialInterval:        time.Second,
	BackoffCoefficient:     2.0,
	MaximumInterval:        time.Second * 100,
	MaximumAttempts:        10,
	NonRetryableErrorTypes: []string{"S3UriParseError"},
}

var DefaultActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: time.Minute,
	RetryPolicy:         &DefaultRetryPolicy,
}

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

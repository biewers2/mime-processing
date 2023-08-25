package core

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"log"
	"net/url"
)

type Unit = struct{}

const ProcessTaskQueue = "mime-processing"

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

func ParseS3Uri(s3uri string) (bucket string, key string, err error) {
	u, err := url.Parse(s3uri)
	if err != nil {
		return "", "", err
	}
	return u.Host, u.Path, nil
}

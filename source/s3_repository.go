package source

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sirupsen/logrus"
)

type S3Repository struct {
	lastUpdateSeconds int64
	data              string
	bucket            string
	key               string
	region            string
}

func (s *S3Repository) GetData(ctx context.Context) (string, error) {
	if ((time.Now().Unix() - s.lastUpdateSeconds) < 10) && s.data != "" {
		logrus.Debug("returning cached file")
		return s.data, nil
	}
	logrus.Debug("fetching file")

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(s.region),
	})
	if err != nil {
		logrus.Debug("error creating session")
		return "", err
	}

	svc := s3.New(sess)
	resp, err := svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	if err != nil {
		logrus.Debug("error getting object")
		return "", err
	}
	defer resp.Body.Close()

	logrus.Debug("reading file")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Debug("error reading file")
		return "", err
	}

	logrus.Debug("setting data")
	s.data = string(data)
	logrus.Debug("setting lastUpdateSeconds")
	s.lastUpdateSeconds = time.Now().Unix()
	return s.data, nil
}

func (s *S3Repository) GetType() string {
	return "s3"
}

func (s *S3Repository) GetPath() string {
	return s.bucket + "/" + s.key
}

func (s *S3Repository) GetUrl() *url.URL {
	return nil
}

func NewS3Repository(bucket, key, region string) (Repository, error) {
	return &S3Repository{bucket: bucket, key: key}, nil
}

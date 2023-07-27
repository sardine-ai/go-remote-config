package source

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"net/url"
	"sync"
)

// S3Repository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file on Amazon S3.
type S3Repository struct {
	sync.RWMutex                        // RWMutex to synchronize access to data during refresh
	data         map[string]interface{} // Map to store the configuration data
	Bucket       string                 // S3 bucket name
	Path         string                 // S3 object key (path to the YAML file within the bucket)
	Region       string                 // AWS region where the S3 bucket is located
}

// Refresh reads the YAML file from Amazon S3, unmarshals it into the data map
func (s *S3Repository) Refresh() error {
	s.Lock()
	defer s.Unlock()

	// Create an AWS session using the specified region.
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(s.Region),
	})
	if err != nil {
		logrus.Debug("error creating session")
		return err
	}

	// Create an S3 client using the session.
	svc := s3.New(sess)

	// Get the object (YAML file) from the specified S3 bucket and object key.
	resp, err := svc.GetObjectWithContext(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.Path),
	})
	if err != nil {
		logrus.Debug("error getting object")
		return err
	}
	defer resp.Body.Close()

	// Read the file content from the response body.
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Debug("error reading file")
		return err
	}

	// Unmarshal the YAML data into the data map.
	err = yaml.Unmarshal(data, &s.data)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}
	return nil
}

// GetData returns a copy of the configuration data stored in the S3Repository.
func (s *S3Repository) GetData() map[string]interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.data
}

// GetType returns the type of the repository (in this case, "s3").
func (s *S3Repository) GetType() string {
	return "s3"
}

// GetPath returns the S3 bucket and object key (path) of the YAML file.
func (s *S3Repository) GetPath() string {
	return s.Bucket + "/" + s.Path
}

// GetUrl returns nil as there is no URL associated with S3Repository.
func (s *S3Repository) GetUrl() *url.URL {
	return nil
}

// NewS3Repository creates a new S3Repository with the provided S3 bucket, object key (path), and AWS region.
func NewS3Repository(bucket, key, region string) (Repository, error) {
	// Create and return a new S3Repository with the specified S3 bucket, object key, and region.
	return &S3Repository{Bucket: bucket, Path: key, Region: region}, nil
}

package source

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"
)

// AwsS3Repository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file within an S3 bucket.
type AwsS3Repository struct {
	sync.RWMutex                         // RWMutex to synchronize access to data during refresh
	Name          string                 // Name of the configuration source
	data          map[string]interface{} // Map to store the configuration data
	BucketName    string                 // Name of the S3 bucket
	ObjectName    string                 // Name of the YAML file within the S3 bucket
	Client        *s3.Client             // S3 client instance
	rawData       []byte                 // Raw data of the YAML configuration file
	clientOnce    sync.Once              // Ensures client is initialized only once
	clientInitErr error                  // Stores error from client initialization
}

// Refresh reads the YAML file from the S3 bucket, unmarshal it into the data map.
func (a *AwsS3Repository) Refresh() error {
	ctx := context.Background()

	// Thread-safe client initialization using sync.Once (only if client not pre-configured)
	if a.Client == nil {
		a.clientOnce.Do(func() {
			cfg, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				a.clientInitErr = fmt.Errorf("failed to load AWS config: %w", err)
				return
			}
			a.Client = s3.NewFromConfig(cfg)
		})
		if a.clientInitErr != nil {
			return a.clientInitErr
		}
	}

	// Network I/O outside lock for better performance
	result, err := a.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.BucketName),
		Key:    aws.String(a.ObjectName),
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	// Read the file content from the reader.
	fileContent, err := io.ReadAll(result.Body)
	if err != nil {
		return err
	}

	// Unmarshal to temp variable outside lock to prevent data corruption on error
	var tempData map[string]interface{}
	err = yaml.Unmarshal(fileContent, &tempData)
	if err != nil {
		return err
	}

	// Only lock for atomic data swap
	a.Lock()
	a.data = tempData
	a.rawData = fileContent
	a.Unlock()

	return nil
}

// GetName returns the name of the configuration source.
func (a *AwsS3Repository) GetName() string {
	return a.Name
}

// GetData returns the configuration data as a map of configuration names to their respective models.
func (a *AwsS3Repository) GetData(configName string) (config interface{}, isPresent bool) {
	a.RLock()
	defer a.RUnlock()
	config, isPresent = a.data[configName]
	return config, isPresent
}

// GetRawData returns the raw data of the YAML configuration file.
func (a *AwsS3Repository) GetRawData() []byte {
	a.RLock()
	defer a.RUnlock()
	return a.rawData
}

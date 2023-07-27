package source

import (
	"cloud.google.com/go/storage"
	"context"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"
	"io"
	"sync"
)

// GCSRepository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file on Google Cloud Storage (GCS).
type GCSRepository struct {
	sync.RWMutex                        // RWMutex to synchronize access to data during refresh
	data         map[string]interface{} // Map to store the configuration data
	Bucket       string                 // GCS bucket name
	Path         string                 // GCS file path
}

// Refresh reads the YAML file from GCS, unmarshals it into the data map.
func (g *GCSRepository) Refresh() error {
	g.Lock()
	defer g.Unlock()

	// Create a new Google Cloud Storage client.
	client, err := storage.NewClient(context.Background(), option.WithoutAuthentication())
	if err != nil {
		logrus.Debug("error creating client")
		return err
	}
	defer client.Close()

	// Get the GCS bucket and object for the specified path.
	bucket := client.Bucket(g.Bucket)
	obj := bucket.Object(g.Path)

	// Create a reader to read the file from GCS.
	reader, err := obj.NewReader(context.Background())
	if err != nil {
		logrus.Debug("error creating reader")
		return err
	}
	defer reader.Close()

	// Read the file data from the reader.
	data, err := io.ReadAll(reader)
	if err != nil {
		logrus.Debug("error reading file")
		return err
	}

	// Unmarshal the YAML data into the data map.
	err = yaml.Unmarshal(data, &g.data)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}

	return nil
}

// GetData returns a copy of the configuration data stored in the GCSRepository.
func (g *GCSRepository) GetData() map[string]interface{} {
	g.RLock()
	defer g.RUnlock()
	return g.data
}

// NewGCSRepository creates a new GCSRepository with the provided GCS bucket and file path.
func NewGCSRepository(bucket, path string) (Repository, error) {
	// Create and return a new GCSRepository with the specified GCS bucket and file path.
	return &GCSRepository{Bucket: bucket, Path: path}, nil
}

package source

import (
	// ...
	"cloud.google.com/go/storage"
	"context"
	"gopkg.in/yaml.v3"
	"io"
	"sync"
	// ...
)

// GcpStorageRepository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file within a GCS bucket.
type GcpStorageRepository struct {
	sync.RWMutex                        // RWMutex to synchronize access to data during refresh
	Name         string                 // Name of the configuration source
	data         map[string]interface{} // Map to store the configuration data
	BucketName   string                 // Name of the GCS bucket
	ObjectName   string                 // Name of the YAML file within the GCS bucket
	Client       *storage.Client        // GCS client instance
	rawData      []byte                 // Raw data of the YAML configuration file
}

// Refresh reads the YAML file from the GCS bucket, unmarshal it into the data map.
func (g *GcpStorageRepository) Refresh() error {
	g.Lock()
	defer g.Unlock()

	// If the GCS client does not exist, create it.
	if g.Client == nil {
		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		if err != nil {
			return err
		}
		g.Client = client
	}

	// Open the YAML file from the GCS bucket.
	ctx := context.Background()
	bucket := g.Client.Bucket(g.BucketName)
	obj := bucket.Object(g.ObjectName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Read the file content from the reader.
	fileContent, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Unmarshal the YAML data into the data map.
	err = yaml.Unmarshal(fileContent, &g.data)
	if err != nil {
		return err
	}

	// Store the raw data of the YAML file.
	g.rawData = fileContent

	return nil
}

// GetName returns the name of the configuration source.
func (g *GcpStorageRepository) GetName() string {
	return g.Name
}

// GetData returns the configuration data as a map of configuration names to their respective models.
func (g *GcpStorageRepository) GetData(configName string) (config interface{}, isPresent bool) {
	g.RLock()
	defer g.RUnlock()
	config, isPresent = g.data[configName]
	return config, isPresent
}

// GetRawData returns the raw data of the YAML configuration file.
func (g *GcpStorageRepository) GetRawData() []byte {
	g.RLock()
	defer g.RUnlock()
	return g.rawData
}

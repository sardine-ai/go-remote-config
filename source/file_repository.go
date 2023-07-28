package source

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"sync"
)

// FileRepository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file.
type FileRepository struct {
	sync.RWMutex                        // RWMutex to synchronize access to data during refresh
	Name         string                 // Name of the configuration source
	Path         string                 // File path of the YAML configuration file
	data         map[string]interface{} // Map to store the configuration data
	rawData      []byte                 // Raw data of the YAML configuration file
}

// GetName returns the name of the configuration source.
func (f *FileRepository) GetName() string {
	return f.Name
}

// GetData returns the configuration data as a map of configuration names to their respective models.
func (f *FileRepository) GetData(configName string) (config interface{}, isPresent bool) {
	f.RLock()
	defer f.RUnlock()
	config, isPresent = f.data[configName]
	return config, isPresent
}

// GetRawData returns the raw data of the YAML configuration file.
func (f *FileRepository) GetRawData() []byte {
	f.RLock()
	defer f.RUnlock()
	return f.rawData
}

// Refresh reads the YAML file, unmarshals it into the data map.
func (f *FileRepository) Refresh() error {
	f.Lock()
	defer f.Unlock()

	// Read the YAML file
	data, err := os.ReadFile(f.Path)
	if err != nil {
		logrus.Debug("error reading file")
		return err
	}

	// Unmarshal the YAML data into the data map
	err = yaml.Unmarshal(data, &f.data)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}

	// Store the raw data of the YAML file
	f.rawData = data

	return nil
}

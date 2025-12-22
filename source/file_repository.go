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

// Refresh reads the YAML file, unmarshal it into the data map.
func (f *FileRepository) Refresh() error {
	// Read the YAML file (no lock needed for read)
	data, err := os.ReadFile(f.Path)
	if err != nil {
		logrus.Debug("error reading file")
		return err
	}

	// Unmarshal to temp variable outside lock to prevent data corruption on error
	var tempData map[string]interface{}
	err = yaml.Unmarshal(data, &tempData)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}

	// Only lock for atomic data swap
	f.Lock()
	f.data = tempData
	f.rawData = data
	f.Unlock()

	return nil
}

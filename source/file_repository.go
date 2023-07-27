package source

import (
	"github.com/divakarmanoj/go-remote-config-server/model"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

// FileRepository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file.
type FileRepository struct {
	sync.RWMutex                         // RWMutex to synchronize access to data during refresh
	Path         string                  // File path of the YAML configuration file
	Url          *url.URL                // URL representation of the file path
	data         map[string]model.Config // Map to store the configuration data
}

// GetData returns a copy of the configuration data stored in the FileRepository.
func (f *FileRepository) GetData() map[string]model.Config {
	f.RLock()
	defer f.RUnlock()
	return f.data
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

	return nil
}

// NewFileRepository creates a new FileRepository with the provided file path.
// It converts the file path to an absolute path and creates a URL representation
// for the file.
func NewFileRepository(path string) (Repository, error) {
	// Convert the file path to a URL representation
	toURL, err := filePathToURL(path)
	if err != nil {
		return nil, err
	}

	// Convert the file path to an absolute path
	path, err = makeAbsoluteFilePath(path)
	if err != nil {
		return nil, err
	}

	// Create and return a new FileRepository with the absolute path and URL.
	return &FileRepository{Path: path, Url: toURL}, nil
}

// filePathToURL converts a file path to a file URL representation.
func filePathToURL(filePath string) (*url.URL, error) {
	// Convert the file path to an absolute path, if it's not already absolute.
	absPath, err := makeAbsoluteFilePath(filePath)
	if err != nil {
		return nil, err
	}

	// Create a URL from the absolute path.
	fileURL := &url.URL{
		Scheme: "file",
		Path:   absPath,
	}

	return fileURL, nil
}

// makeAbsoluteFilePath converts the input file path to an absolute path.
func makeAbsoluteFilePath(filePath string) (string, error) {
	// Convert the input file path to an absolute path.
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		logrus.WithError(err).Error("error getting absolute path")
		return "", err
	}

	return absPath, nil
}

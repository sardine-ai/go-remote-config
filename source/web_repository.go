package source

import (
	"context"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/url"
	"sync"
)

// WebRepository is a struct that implements the Repository interface for
// handling configuration data fetched from a remote HTTP endpoint (web URL).
type WebRepository struct {
	sync.RWMutex                        // RWMutex to synchronize access to data during refresh
	data         map[string]interface{} // Map to store the configuration data
	URL          *url.URL               // URL representing the remote HTTP endpoint (web URL)
}

// GetData returns the configuration data as a map of configuration names to their respective models.
func (w *WebRepository) GetData(configName string) (config interface{}, isPresent bool) {
	w.RLock()
	defer w.RUnlock()
	config, isPresent = w.data[configName]
	return config, isPresent
}

// Refresh fetches the YAML file from the remote HTTP endpoint (web URL),
// unmarshal it into the data map.
func (w *WebRepository) Refresh() error {
	w.Lock()
	defer w.Unlock()

	// Create an HTTP request to fetch the YAML file from the remote web URL.
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, w.URL.String(), nil)
	if err != nil {
		logrus.Debug("error creating request")
		return err
	}

	// Perform the HTTP request to get the YAML file content.
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		logrus.Debug("error doing request")
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
	err = yaml.Unmarshal(data, &w.data)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}

	return nil
}

// NewWebRepository creates a new WebRepository with the provided web URL.
func NewWebRepository(webURL string) (Repository, error) {
	// Parse the web URL into a URL representation.
	parsedURL, err := url.Parse(webURL)
	if err != nil {
		return nil, err
	}
	// Create and return a new WebRepository with the specified web URL.
	return &WebRepository{URL: parsedURL}, nil
}

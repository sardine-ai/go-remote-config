package source

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"net/url"
	"os"
	"sync"
)

// GitRepository is a struct that implements the Repository interface for
// handling configuration data stored in a YAML file within a Git repository.
// Deprecated: This is Deprecated because it there is API limitation you make to github and gitlab. Which will get exhausted.
// This is not a good way to handle the configuration is to use your CI to upload the configuration to a S3/GCS bucket and then use the S3/GCS  repository to fetch the configuration.
type GitRepository struct {
	sync.RWMutex                         // RWMutex to synchronize access to data during refresh
	Name          string                 // Name of the configuration source
	data          map[string]interface{} // Map to store the configuration data
	URL           *url.URL               // URL representing the Git repository URL
	Path          string                 // Path to the YAML file within the Git repository
	gitRepository *git.Repository        // Go-Git repository instance for the in-memory clone
	Branch        string                 // Branch to use when cloning the Git repository
	Auth          *http.BasicAuth        // BasicAuth to use when cloning the Git repository
	fs            billy.Filesystem       // Filesystem to store the in-memory clone of the repository
	rawData       []byte                 // Raw data of the YAML configuration file
}

// GetName returns the configuration data as a map of configuration names to their respective models.
func (g *GitRepository) GetName() string {
	return g.Name
}

// GetRawData returns the raw data of the YAML configuration file.
func (g *GitRepository) GetRawData() []byte {
	g.RLock()
	defer g.RUnlock()
	return g.rawData
}

// Refresh reads the YAML file from the Git repository, unmarshal it into the data map.
func (g *GitRepository) Refresh() error {
	g.Lock()
	defer g.Unlock()

	// If the in-memory clone of the Git repository does not exist, create it.
	if g.fs == nil {
		g.fs = memfs.New()
		logrus.Debugf("Cloning %s into memory", g.URL.String())
		// Clone the Git repository into the in-memory filesystem.
		r, err := git.CloneContext(context.Background(), memory.NewStorage(), g.fs, &git.CloneOptions{
			URL:  g.URL.String(),
			Auth: g.Auth,
		})
		if err != nil {
			return err
		}

		if g.Branch != "" {
			// Checkout the specified Branch.
			w, err := r.Worktree()
			if err != nil {
				return err
			}

			err = r.Fetch(&git.FetchOptions{
				RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
			})
			if err != nil {
				return err
			}

			err = w.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName(g.Branch),
				Force:  true,
			})
			if err != nil {
				return err
			}
		}

		logrus.Debug("Cloned")
		g.gitRepository = r
	} else {
		// Pull the latest changes from the Git repository.
		w, err := g.gitRepository.Worktree()
		if err != nil {
			return err
		}
		logrus.Debug("Pulling")

		pullOptions := &git.PullOptions{
			Auth: g.Auth,
		}
		if g.Branch != "" {
			pullOptions = &git.PullOptions{
				ReferenceName: plumbing.NewBranchReferenceName(g.Branch),
				Force:         true,
				SingleBranch:  true,
				Auth:          g.Auth,
			}
		}

		err = w.PullContext(context.Background(), pullOptions)

		if err != nil && err != git.NoErrAlreadyUpToDate {
			return err
		}
		if err == git.NoErrAlreadyUpToDate {
			logrus.Debug("Already up to date")
		} else {
			logrus.Debug("Pulled")
		}
	}

	// Open the YAML file from the in-memory filesystem.
	file, err := g.fs.Open(g.Path)
	if err != nil {
		fmt.Println("Error opening file:", err)
		os.Exit(1)
	}
	defer func(file billy.File) {
		err := file.Close()
		if err != nil {
			logrus.WithError(err).Error("error closing file")
		}
	}(file)

	// Read the file content from the reader.
	fileContent, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	// Unmarshal the YAML data into the data map.
	err = yaml.Unmarshal(fileContent, &g.data)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}

	// Store the raw data of the YAML file.
	g.rawData = fileContent

	return nil
}

// GetData returns the configuration data as a map of configuration names to their respective models.
// Deprecated: This is Deprecated because it there is API limitation you make to github and gitlab. Which will get exhausted.
// This is not a good way to handle the configuration is to use your CI to upload the configuration to a S3/GCS bucket and then use the S3/GCS  repository to fetch the configuration.
func (g *GitRepository) GetData(configName string) (config interface{}, isPresent bool) {
	g.RLock()
	defer g.RUnlock()
	config, isPresent = g.data[configName]
	return config, isPresent
}

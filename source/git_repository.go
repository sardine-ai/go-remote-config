package source

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
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
	cloneOnce     sync.Once              // Ensures repository is cloned only once
	cloneErr      error                  // Stores error from clone operation
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
	ctx := context.Background()

	// Thread-safe clone using sync.Once (only first call clones)
	g.cloneOnce.Do(func() {
		g.fs = memfs.New()
		logrus.Debugf("Cloning %s into memory", g.URL.String())
		r, err := git.CloneContext(ctx, memory.NewStorage(), g.fs, &git.CloneOptions{
			URL:  g.URL.String(),
			Auth: g.Auth,
		})
		if err != nil {
			g.cloneErr = err
			return
		}

		if g.Branch != "" {
			w, err := r.Worktree()
			if err != nil {
				g.cloneErr = err
				return
			}

			err = r.Fetch(&git.FetchOptions{
				RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
			})
			if err != nil {
				g.cloneErr = err
				return
			}

			err = w.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName(g.Branch),
				Force:  true,
			})
			if err != nil {
				g.cloneErr = err
				return
			}
		}

		logrus.Debug("Cloned")
		g.gitRepository = r
	})
	if g.cloneErr != nil {
		return g.cloneErr
	}

	// Pull latest changes (no lock needed - idempotent operation)
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

	err = w.PullContext(ctx, pullOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	if err == git.NoErrAlreadyUpToDate {
		logrus.Debug("Already up to date")
	} else {
		logrus.Debug("Pulled")
	}

	// Read the config file
	file, err := g.fs.Open(g.Path)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", g.Path, err)
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", g.Path, err)
	}

	// Unmarshal to temp variable outside lock to prevent data corruption on error
	var tempData map[string]interface{}
	err = yaml.Unmarshal(fileContent, &tempData)
	if err != nil {
		logrus.Debug("error unmarshalling file")
		return err
	}

	// Only lock for atomic data swap
	g.Lock()
	g.data = tempData
	g.rawData = fileContent
	g.Unlock()

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

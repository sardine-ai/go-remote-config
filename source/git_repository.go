package source

import (
	"context"
	"fmt"
	"github.com/divakarmanoj/go-remote-config-server/model"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
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
type GitRepository struct {
	sync.RWMutex                          // RWMutex to synchronize access to data during refresh
	data          map[string]model.Config // Map to store the configuration data
	URL           *url.URL                // URL representing the Git repository URL
	Path          string                  // Path to the YAML file within the Git repository
	GitRepository *git.Repository         // Go-Git repository instance for the in-memory clone
	fs            billy.Filesystem        // Filesystem to store the in-memory clone of the repository
}

// Refresh reads the YAML file from the Git repository, unmarshals it into the data map.
func (g *GitRepository) Refresh() error {
	g.Lock()
	defer g.Unlock()

	// If the in-memory clone of the Git repository does not exist, create it.
	if g.fs == nil {
		g.fs = memfs.New()
		logrus.Debugf("Cloning %s into memory", g.URL.String())
		// Clone the Git repository into the in-memory filesystem.
		r, err := git.CloneContext(context.Background(), memory.NewStorage(), g.fs, &git.CloneOptions{
			URL: g.URL.String(),
		})
		if err != nil {
			return err
		}
		logrus.Debug("Cloned")
		g.GitRepository = r
	} else {
		// Pull the latest changes from the Git repository.
		w, err := g.GitRepository.Worktree()
		if err != nil {
			return err
		}
		logrus.Debug("Pulling")
		err = w.PullContext(context.Background(), &git.PullOptions{})
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
	defer file.Close()

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

	return nil
}

// GetData returns a copy of the configuration data stored in the GitRepository.
func (g *GitRepository) GetData() map[string]model.Config {
	g.RLock()
	defer g.RUnlock()
	return g.data
}

// NewGitRepository creates a new GitRepository with the provided Git URL and file path.
func NewGitRepository(gitURL string, path string) (Repository, error) {
	// Parse the Git URL into a URL representation.
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		return nil, err
	}
	// Create and return a new GitRepository with the Git URL and file path.
	return &GitRepository{URL: parsedURL, Path: path}, nil
}

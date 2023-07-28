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
type GitRepository struct {
	sync.RWMutex                         // RWMutex to synchronize access to data during refresh
	data          map[string]interface{} // Map to store the configuration data
	URL           *url.URL               // URL representing the Git repository URL
	Path          string                 // Path to the YAML file within the Git repository
	gitRepository *git.Repository        // Go-Git repository instance for the in-memory clone
	Branch        string                 // Branch to use when cloning the Git repository
	Auth          *http.BasicAuth        // BasicAuth to use when cloning the Git repository
	fs            billy.Filesystem       // Filesystem to store the in-memory clone of the repository
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
		// Get the HEAD reference, which points to the current branch
		ref, err := g.gitRepository.Head()
		if err != nil {
			fmt.Printf("Error getting HEAD reference: %v\n", err)
			os.Exit(1)
		}

		// Get the name of the current branch from the reference
		branchName := ref.Name().Short()

		fmt.Printf("Current branch: %s\n", branchName)
		fmt.Printf("Current branch: %s\n", branchName)
		fmt.Printf("Current branch: %s\n", branchName)
		fmt.Printf("Current branch: %s\n", branchName)
		fmt.Printf("Current branch: %s\n", branchName)
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

// GetData returns the configuration data as a map of configuration names to their respective models.
func (g *GitRepository) GetData(configName string) (config interface{}, isPresent bool) {
	g.RLock()
	defer g.RUnlock()
	config, isPresent = g.data[configName]
	return config, isPresent
}

// NewGitRepository creates a new GitRepository with the provided Git URL and file path.
func NewGitRepository(gitURL string, path string) (Repository, error) {
	// Parse the Git URL into a URL representation.
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		return nil, err
	}
	// Create and return a new gitRepository with the Git URL and file path.
	return &GitRepository{URL: parsedURL, Path: path}, nil
}

package source

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
)

type GitRepository struct {
	lastUpdateSeconds int64
	data              string
	url               *url.URL
	path              string
	repo              *git.Repository
	fs                billy.Filesystem
}

func (g *GitRepository) GetUrl() *url.URL {
	return g.url
}

func (g *GitRepository) GetData(ctx context.Context) (string, error) {
	if ((time.Now().Unix() - g.lastUpdateSeconds) < 10) && g.data != "" {
		logrus.Debug("returning cached file")
		return g.data, nil
	}

	if g.fs == nil {
		g.fs = memfs.New()
		logrus.Debugf("Cloning %s into memory", g.url.String())
		// Clone the Git repository
		r, err := git.CloneContext(ctx, memory.NewStorage(), g.fs, &git.CloneOptions{
			URL: g.url.String(),
		})
		if err != nil {
			return "", err
		}
		logrus.Debug("Cloned")
		g.repo = r
	} else {
		// Pull the latest changes from the Git repository
		w, err := g.repo.Worktree()
		if err != nil {
			return "", err
		}
		logrus.Debug("Pulling")
		err = w.PullContext(ctx, &git.PullOptions{})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return "", err
		}
		if err == git.NoErrAlreadyUpToDate {
			logrus.Debug("Already up to date")
		} else {
			logrus.Debug("Pulled")
		}
	}

	file, err := g.fs.Open(g.path)

	fileContent, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	g.data = string(fileContent)
	g.lastUpdateSeconds = time.Now().Unix()
	return g.data, nil
}

func (g *GitRepository) GetType() string {
	return "git"
}

func (g *GitRepository) GetPath() string {
	return g.path
}

func NewGitRepository(gitUrl string, path string) (Repository, error) {
	parsedUrl, err := url.Parse(gitUrl)
	if err != nil {
		return nil, err
	}
	return &GitRepository{url: parsedUrl, path: path}, nil
}

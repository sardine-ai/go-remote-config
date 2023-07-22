package main

import (
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

func (g *GitRepository) getUrl() *url.URL {
	return g.url
}

func (g *GitRepository) getData() (string, error) {
	if ((time.Now().Unix() - g.lastUpdateSeconds) < 10) && g.data != "" {
		logrus.Debug("returning cached file")
		return g.data, nil
	}

	if g.fs == nil {
		g.fs = memfs.New()
		// Clone the Git repository
		r, err := git.Clone(memory.NewStorage(), g.fs, &git.CloneOptions{
			URL: g.url.String(),
		})
		if err != nil {
			return "", err
		}
		g.repo = r
	} else {
		// Pull the latest changes from the Git repository
		w, err := g.repo.Worktree()
		if err != nil {
			return "", err
		}
		err = w.Pull(&git.PullOptions{})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return "", err
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

func (g *GitRepository) getType() string {
	return "git"
}

func (g *GitRepository) getPath() string {
	return g.path
}

func NewGitRepository(gitUrl string, path string) (Repository, error) {
	parsedUrl, err := url.Parse(gitUrl)
	if err != nil {
		return nil, err
	}
	return &GitRepository{url: parsedUrl, path: path}, nil
}

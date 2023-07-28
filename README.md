# go-remote-config
![go build and test](https://github.com/divakarmanoj/go-remote-config/actions/workflows/go.yml/badge.svg)

this is a simple remote config for golang. It supports yaml and json files. It also supports local Files, github repositories and web urls.

### Usage
FileRepository
```go
package main

import (
	"context"
	"github.com/divakarmanoj/go-remote-config/client"
	"github.com/divakarmanoj/go-remote-config/source"
	"time"
)

func main() {
	repository := source.FileRepository{
		Path: "config.yaml",
		Name: "config",
	}
	ctx := context.Background()
	client := client.NewClient(ctx, &repository, 10*time.Second)
	var name string
	err := client.GetConfig("name", &name)
	if err != nil {
		return
	}
}

```

Web Repository
```go
package main

import (
	"context"
	"github.com/divakarmanoj/go-remote-config/client"
	"github.com/divakarmanoj/go-remote-config/source"
	"net/url"
	"time"
)

func main() {
	urlParsed, err := url.Parse("https://raw.githubusercontent.com/divakarmanoj/go-remote-config/go-only/test.yaml")
	if err != nil {
		return
	}
	repository := source.WebRepository{
		URL:  urlParsed,
		Name: "config",
	}
	ctx := context.Background()
	client := client.NewClient(ctx, &repository, 10*time.Second)
	var name string
	err = client.GetConfig("name", &name)
	if err != nil {
		return
	}
	println(name)
}
```
Github Repository
```go
package main

import (
	"context"
	"github.com/divakarmanoj/go-remote-config/client"
	"github.com/divakarmanoj/go-remote-config/source"
	"net/url"
	"time"
)

func main() {
	urlParsed, err := url.Parse("https://github.com/divakarmanoj/go-remote-config.git")
	if err != nil {
		return
	}
	repository := source.GitRepository{
		URL:    urlParsed,
		Path:   "test.yaml",
		Branch: "go-only",
	}
	ctx := context.Background()
	ConfigClient := client.NewClient(ctx, &repository, 10*time.Second)
	var name string
	err = ConfigClient.GetConfig("name", &name)
	if err != nil {
		return
	}
	println(name)
}
```

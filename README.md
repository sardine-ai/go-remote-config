# go-remote-config-server

## Description
This is a simple remote config server written in Go. It is intended to be used as a remote config server for web and mobile applications.
The configuration data is in Yaml format

## Usage
 There are two ways data sources used to define the configuration data.
 - File system
```bash
go run main.go --path /path/to/config/dir --repo-type fs
```
 - Git repository
```bash
go run main.go --url enter_url_here --repo-type git
```
for now only http is supported for git repositories and the url must be a public repository
 - HTTP
```bash
go run main.go --url enter_url_here --repo-type http
```
the url must be a public url
 
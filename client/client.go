package client

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sardine-ai/go-remote-config/source"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Client manages configuration data from a repository with automatic refresh.
type Client struct {
	Repository      source.Repository
	RefreshInterval time.Duration
	cancel          context.CancelFunc

	// Thread-safe closed state using atomic operations
	closed atomic.Bool

	// Staleness tracking for refresh failures
	mu              sync.RWMutex
	lastRefreshTime time.Time
	lastRefreshErr  error
	refreshCount    int64
	refreshErrors   int64
}

var (
	defaultClient   *Client
	defaultClientMu sync.RWMutex
)

// NewClient creates a new Client with the provided context, repository,
// and refresh interval. It starts a background goroutine to periodically
// refresh the configuration data from the repository based on the given
// refresh interval. The new client is automatically set as the default client.
// Use NewClientWithOptions if you need more control over this behavior.
func NewClient(ctx context.Context, repository source.Repository, refreshInterval time.Duration) (*Client, error) {
	return NewClientWithOptions(ctx, repository, refreshInterval, DefaultClientOptions())
}

// SetDefaultClient sets the default client to the provided client.
// This is useful when you want to use a specific client as the default
// without creating a new one via NewClient. This function is thread-safe.
func SetDefaultClient(client *Client) {
	defaultClientMu.Lock()
	defaultClient = client
	defaultClientMu.Unlock()
}

// ClientOptions contains options for creating a new Client.
type ClientOptions struct {
	// SetAsDefault determines whether the new client should be set as the
	// default client for package-level functions like GetConfig().
	// Defaults to true for backwards compatibility with NewClient().
	SetAsDefault bool
}

// DefaultClientOptions returns the default options used by NewClient().
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		SetAsDefault: true,
	}
}

// NewClientWithOptions creates a new Client with the provided context, repository,
// refresh interval, and options. Unlike NewClient, this function allows you to
// control whether the new client is set as the default client.
//
// Use this when you need multiple independent clients that don't interfere
// with each other's default client state.
func NewClientWithOptions(ctx context.Context, repository source.Repository, refreshInterval time.Duration, opts ClientOptions) (*Client, error) {
	// Create a new context and its corresponding cancel function
	// for the Client. This allows us to control the lifetime of the
	// background refresh goroutine.
	ctx, cancel := context.WithCancel(ctx)

	// Create the Client instance with the provided repository and refresh interval.
	client := &Client{
		Repository:      repository,
		RefreshInterval: refreshInterval,
		cancel:          cancel,
	}

	// Refresh the configuration data for the first time to ensure the
	// Client is initialized with the latest data before it is used.
	err := client.Repository.Refresh()
	if err != nil {
		logrus.WithError(err).Error("error refreshing repository")
		client.recordRefreshError(err)
		return nil, err
	}
	client.recordRefreshSuccess()

	// Start the background refresh goroutine
	go refresh(ctx, client)

	// Only set as default if requested
	if opts.SetAsDefault {
		defaultClientMu.Lock()
		defaultClient = client
		defaultClientMu.Unlock()
	}

	return client, nil
}

// refresh is a goroutine that periodically refreshes the configuration data
// from the repository based on the provided refresh interval. It stops
// refreshing when the given context is canceled.
func refresh(ctx context.Context, client *Client) {
	ticker := time.NewTicker(client.RefreshInterval) // Create a new ticker with the given refresh interval
	defer ticker.Stop()                              // Stop the ticker when the goroutine exits to prevent resource leak
	for {
		select {
		case <-ticker.C:
			// The ticker has ticked, indicating it's time to refresh the data
			err := client.Repository.Refresh() // Call the Refresh method of the repository to update the configuration data
			if err != nil {
				logrus.WithError(err).Error("error refreshing repository")
				client.recordRefreshError(err)
			} else {
				client.recordRefreshSuccess()
			}
		case <-ctx.Done():
			// The context is canceled, indicating the refresh routine should stop
			return
		}
	}
}

// recordRefreshSuccess records a successful refresh operation.
func (c *Client) recordRefreshSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRefreshTime = time.Now()
	c.lastRefreshErr = nil
	c.refreshCount++
}

// recordRefreshError records a failed refresh operation.
func (c *Client) recordRefreshError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRefreshErr = err
	c.refreshErrors++
}

// RefreshStatus contains information about the client's refresh state.
type RefreshStatus struct {
	LastRefreshTime time.Time
	LastRefreshErr  error
	RefreshCount    int64
	RefreshErrors   int64
	IsStale         bool
	StaleDuration   time.Duration
}

// GetRefreshStatus returns the current refresh status of the client.
// This is useful for health checks and monitoring.
func (c *Client) GetRefreshStatus() RefreshStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := RefreshStatus{
		LastRefreshTime: c.lastRefreshTime,
		LastRefreshErr:  c.lastRefreshErr,
		RefreshCount:    c.refreshCount,
		RefreshErrors:   c.refreshErrors,
	}

	// Consider stale if last refresh was more than 2x the refresh interval ago
	if !c.lastRefreshTime.IsZero() {
		status.StaleDuration = time.Since(c.lastRefreshTime)
		status.IsStale = status.StaleDuration > (c.RefreshInterval * 2)
	}

	return status
}

// IsHealthy returns true if the client has fresh config data.
// Only checks staleness, not last refresh error, to avoid pod restarts
// on transient errors (e.g., brief S3 outage). Use GetRefreshStatus()
// to check LastRefreshErr for alerting/monitoring purposes.
func (c *Client) IsHealthy() bool {
	if c.closed.Load() {
		return false
	}
	status := c.GetRefreshStatus()
	return !status.IsStale
}

// getDefaultClient returns the default client in a thread-safe manner.
func getDefaultClient() *Client {
	defaultClientMu.RLock()
	defer defaultClientMu.RUnlock()
	return defaultClient
}

func GetConfig(name string, data interface{}, defaultValue interface{}) error {
	client := getDefaultClient()
	if client == nil {
		return errors.New("no default client configured, call NewClient first")
	}
	return client.GetConfig(name, data, defaultValue)
}

func GetConfigArrayOfStrings(name string, defaultValue []string) ([]string, error) {
	client := getDefaultClient()
	if client == nil {
		return defaultValue, errors.New("no default client configured, call NewClient first")
	}
	return client.GetConfigArrayOfStrings(name, defaultValue)
}

func GetConfigString(name string, defaultValue string) (string, error) {
	client := getDefaultClient()
	if client == nil {
		return defaultValue, errors.New("no default client configured, call NewClient first")
	}
	return client.GetConfigString(name, defaultValue)
}

func GetConfigInt(name string, defaultValue int) (int, error) {
	client := getDefaultClient()
	if client == nil {
		return defaultValue, errors.New("no default client configured, call NewClient first")
	}
	return client.GetConfigInt(name, defaultValue)
}

func GetConfigFloat(name string, defaultValue float64) (float64, error) {
	client := getDefaultClient()
	if client == nil {
		return defaultValue, errors.New("no default client configured, call NewClient first")
	}
	return client.GetConfigFloat(name, defaultValue)
}

// Close stops the background refresh goroutine of the Client by canceling
// its associated context. This function allows graceful termination of the
// background routine and prevents potential goroutine leaks. It should be
// called when the Client is no longer needed to release resources properly.
func (c *Client) Close() {
	// Mark the client as closed using atomic operation for thread safety
	c.closed.Store(true)
	// Call the Cancel function associated with the Client's context.
	// This cancels the context, causing the background refresh goroutine
	// (started by NewClient) to return and terminate gracefully.
	c.cancel()
}

// IsClosed returns true if the client has been closed.
// This method is thread-safe.
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// setDefaultValue sets the value pointed to by data to defaultValue using reflection.
// This is needed because Go's value semantics prevent direct assignment to interface{} parameters.
func setDefaultValue(data interface{}, defaultValue interface{}) {
	if defaultValue == nil {
		return
	}
	dataVal := reflect.ValueOf(data)
	if dataVal.Kind() != reflect.Ptr || dataVal.IsNil() {
		return
	}
	defaultVal := reflect.ValueOf(defaultValue)
	if dataVal.Elem().Type() == defaultVal.Type() {
		dataVal.Elem().Set(defaultVal)
	}
}

// GetConfig retrieves the configuration with the given name from the repository
// and stores it in the provided data pointer. It returns an error if the
// configuration is not found, the data argument is not a non-nil pointer, or
// the type of the data is not compatible with the type in the repository.
func (c *Client) GetConfig(name string, data interface{}, defaultValue interface{}) error {
	if c.closed.Load() {
		setDefaultValue(data, defaultValue)
		return errors.New("client is closed")
	}
	// Get the configuration data from the repository
	config, ok := c.Repository.GetData(name)
	if !ok {
		setDefaultValue(data, defaultValue)
		return errors.New("config not found")
	}

	marshal, err := yaml.Marshal(config)
	if err != nil {
		setDefaultValue(data, defaultValue)
		return err
	}
	// Unmarshal the configuration data into the provided data pointer
	err = yaml.Unmarshal(marshal, data)
	if err != nil {
		setDefaultValue(data, defaultValue)
		return err
	}

	return nil
}

// GetConfigArrayOfStrings retrieves the configuration with the given name from the repository
func (c *Client) GetConfigArrayOfStrings(name string, defaultValue []string) ([]string, error) {
	if c.closed.Load() {
		return defaultValue, errors.New("client is closed")
	}
	// Get the configuration data from the repository
	config, ok := c.Repository.GetData(name)
	if !ok {
		return defaultValue, errors.New("config not found")
	}

	configArray, ok := config.([]interface{})
	if !ok {
		return defaultValue, errors.New("config is not an array of strings")
	}
	output := []string{}
	for _, v := range configArray {
		str, ok := v.(string)
		if !ok {
			return defaultValue, errors.New("config is not an array of strings")
		}
		output = append(output, str)
	}

	return output, nil
}

// GetConfigString retrieves the configuration with the given name from the repository
func (c *Client) GetConfigString(name string, defaultValue string) (string, error) {
	if c.closed.Load() {
		return defaultValue, errors.New("client is closed")
	}
	// Get the configuration data from the repository
	config, ok := c.Repository.GetData(name)
	if !ok {
		return defaultValue, errors.New("config not found")
	}

	configString, ok := config.(string)
	if !ok {
		return defaultValue, errors.New("config is not a string")
	}

	return configString, nil
}

// GetConfigInt retrieves the configuration with the given name from the repository
func (c *Client) GetConfigInt(name string, defaultValue int) (int, error) {
	if c.closed.Load() {
		return defaultValue, errors.New("client is closed")
	}
	// Get the configuration data from the repository
	config, ok := c.Repository.GetData(name)
	if !ok {
		return defaultValue, errors.New("config not found")
	}
	configInt, ok := config.(int)
	if !ok {
		return defaultValue, errors.New("config is not an int64")
	}

	return configInt, nil
}

// GetConfigFloat retrieves the configuration with the given name from the repository
func (c *Client) GetConfigFloat(name string, defaultValue float64) (float64, error) {
	if c.closed.Load() {
		return defaultValue, errors.New("client is closed")
	}
	// Get the configuration data from the repository
	config, ok := c.Repository.GetData(name)
	if !ok {
		return defaultValue, errors.New("config not found")
	}
	configInt, ok := config.(float64)
	if !ok {
		return defaultValue, errors.New("config is not an int64")
	}

	return configInt, nil
}

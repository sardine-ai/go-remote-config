package client

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fullstorydev/emulators/storage/gcsemu"

	"github.com/sardine-ai/go-remote-config/source"
)

func TestNewClient(t *testing.T) {
	urlParsed, err := url.Parse("https://raw.githubusercontent.com/sardine-ai/go-remote-config/refs/heads/main/test.yaml")
	if err != nil {
		t.Errorf("Error parsing url: %s", err.Error())
	}

	// start an in-memory Storage test server (for unit tests)
	svr, err := gcsemu.NewServer("127.0.0.1:9023", gcsemu.Options{})
	if err != nil {
		t.Errorf("Error starting in-memory storage server: %s", err.Error())
	}
	defer svr.Close()
	err = os.Setenv("STORAGE_EMULATOR_HOST", "http://127.0.0.1:9023")
	if err != nil {
		t.Errorf("Error setting env variable: %s", err.Error())
	}
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Errorf("Error creating storage client: %s", err.Error())
	}

	bucket := client.Bucket("test-bucket")

	if err := bucket.Create(ctx, "test-project", nil); err != nil {
		log.Fatalf("Failed to create bucket: %v", err)
	}

	object := bucket.Object("test.yaml")

	w := object.NewWriter(ctx)

	// Open the local file to be uploaded.
	data, err := os.ReadFile("../test.yaml")
	if err != nil {
		log.Fatalf("Failed to open the local file: %v", err)
	}

	if _, err := w.Write(data); err != nil {
		log.Fatalf("Failed to upload file: %v", err)
	}

	// Close the GCS writer, flushing any remaining data to GCS.
	if err := w.Close(); err != nil {
		log.Fatalf("Failed to close the GCS writer: %v", err)
	}

	endpointResolverOpt := config.WithEndpointResolver(aws.EndpointResolverFunc(
		func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           "http://localhost:4566",
				SigningRegion: "us-east-1",
			}, nil
		}))
	credentialsProviderOpt := config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID: "dummy", SecretAccessKey: "dummy", SessionToken: "dummy",
			Source: "Hard-coded credentials; values are irrelevant for local AWS services",
		}})

	cfg, err := config.LoadDefaultConfig(ctx, endpointResolverOpt, credentialsProviderOpt, config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("Failed to load AWS client config: %v", err)
	}
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	})
	if err != nil {
		log.Fatalf("Failed to create S3 bucket: %v", err)
	}

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test.yaml"),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		log.Fatalf("Failed to upload file to S3: %v", err)
	}

	testCases := []struct {
		name            string
		repository      source.Repository
		refreshInterval time.Duration
	}{
		{
			name:            "FileRepository",
			repository:      &source.FileRepository{Path: "../test.yaml"},
			refreshInterval: 10 * time.Second,
		},
		{
			name:            "WebRepository",
			repository:      &source.WebRepository{URL: urlParsed},
			refreshInterval: 10 * time.Second,
		},
		//{
		//	name:            "gitRepository",
		//	repository:      &source.GitRepository{URL: gitUrlParsed, Path: "test.yaml", Branch: "go-only"},
		//	refreshInterval: 10 * time.Second,
		//},
		{
			name:            "GcpStorageRepository",
			repository:      &source.GcpStorageRepository{BucketName: "test-bucket", ObjectName: "test.yaml", Client: client},
			refreshInterval: 10 * time.Second,
		},
		{
			name:            "GcpStorageRepository",
			repository:      &source.AwsS3Repository{BucketName: "test-bucket", ObjectName: "test.yaml", Client: s3Client},
			refreshInterval: 10 * time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := NewClient(ctx, tc.repository, tc.refreshInterval)
			var name string
			err = client.GetConfig("name", &name, nil)
			if err != nil {
				t.Errorf("Error getting name: %s", err.Error())
			}
			if name != "John" {
				t.Errorf("Expected name to be John, got %s", name)
			}
			name, err = client.GetConfigString("name", "")
			if err != nil {
				t.Errorf("Error getting name: %s", err.Error())
			}
			if name != "John" {
				t.Errorf("Expected name to be John, got %s", name)
			}
			type Address struct {
				Street  string `yaml:"street"`
				City    string `yaml:"city"`
				Country string `yaml:"country"`
				Zip     string `yaml:"zip_code"`
			}
			var address Address
			err = client.GetConfig("address", &address, nil)
			if err != nil {
				t.Errorf("Error getting address: %s", err.Error())
			}
			if address.Street != "123 Main St" {
				t.Errorf("Expected street to be 123 Main St, got %s", address.Street)
			}
			if address.City != "New York" {
				t.Errorf("Expected city to be New York, got %s", address.City)
			}
			if address.Country != "USA" {
				t.Errorf("Expected country to be USA, got %s", address.Country)
			}
			if address.Zip != "10001" {
				t.Errorf("Expected zip to be 10001, got %s", address.Zip)
			}
			var isEmployee bool
			err = client.GetConfig("is_employee", &isEmployee, nil)
			if err != nil {
				t.Errorf("Error getting is_employee: %s", err.Error())
			}
			if isEmployee != true {
				t.Errorf("Expected is_employee to be true, got %t", isEmployee)
			}
			var hobbies []string
			err = client.GetConfig("hobbies", &hobbies, nil)
			if err != nil {
				t.Errorf("Error getting hobbies: %s", err.Error())
			}
			if !reflect.DeepEqual(hobbies, []string{"Reading", "Cooking", "Hiking", "Swimming", "Coding"}) {
				t.Errorf("Expected hobbies to contain Reading, Cooking, Hiking, Swimming, Coding, got %v", hobbies)
			}
			hobbies, err = client.GetConfigArrayOfStrings("hobbies", nil)
			if err != nil {
				t.Errorf("Error getting hobbies: %s", err.Error())
			}
			if !reflect.DeepEqual(hobbies, []string{"Reading", "Cooking", "Hiking", "Swimming", "Coding"}) {
				t.Errorf("Expected hobbies to contain Reading, Cooking, Hiking, Swimming, Coding, got %v", hobbies)
			}
			var age int64
			err = client.GetConfig("age", &age, nil)
			if err != nil {
				t.Errorf("Error getting age: %s", err.Error())
			}
			if age != 30 {
				t.Errorf("Expected age to be 30, got %d", age)
			}
			var intAge int
			intAge, err = client.GetConfigInt("age", 0)
			if intAge != 30 {
				t.Errorf("Expected age to be 30, got %d", intAge)
			}
			var floatAge float64
			floatAge, err = client.GetConfigFloat("float_age", 0)
			if floatAge != 303984756986439880155862132370440192 {
				t.Errorf("Expected age to be 30, got %f", floatAge)
			}
			client.Close()
		})
	}
}

type test struct {
	ShouldError    bool
	GetRefeshCount int
}

func (t *test) GetData(_ string) (config interface{}, isPresent bool) {
	return t.GetRefeshCount, true
}

func (t *test) GetRawData() []byte {
	return []byte("test")
}

func (t *test) Refresh() error {
	t.GetRefeshCount = t.GetRefeshCount + 1
	if t.ShouldError {
		return errors.New("error")
	}
	return nil
}

func (t *test) GetName() string {
	return "test"
}

func TestRefresh(t *testing.T) {
	// should throw Err
	_, err := NewClient(context.Background(), &test{ShouldError: true}, 1*time.Second)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	// should not throw Err
	client := &Client{Repository: &test{ShouldError: false}, RefreshInterval: 1 * time.Second}
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	var count int
	if client.GetConfig("test", &count, nil) != nil {
		t.Errorf("Expected error, got nil")
	}
	if count != 0 {
		t.Errorf("Expected count to be 0, got %d", count)
	}
	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	refresh(ctx, client)
	if client.GetConfig("test", &count, nil) != nil {
		t.Errorf("Expected error, got nil")
	}
	if count != 1 {
		t.Errorf("Expected count to be 1, got %d", count)
	}
}

// mockRepository is a thread-safe mock repository for testing
type mockRepository struct {
	mu           sync.RWMutex
	data         map[string]interface{}
	refreshCount int
	shouldError  bool
	refreshDelay time.Duration
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		data: map[string]interface{}{
			"name": "test",
			"age":  30,
		},
	}
}

func (m *mockRepository) GetName() string {
	return "mock"
}

func (m *mockRepository) GetData(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *mockRepository) GetRawData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return []byte("name: test\nage: 30")
}

func (m *mockRepository) Refresh() error {
	if m.refreshDelay > 0 {
		time.Sleep(m.refreshDelay)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshCount++
	if m.shouldError {
		return errors.New("mock refresh error")
	}
	return nil
}

func (m *mockRepository) getRefreshCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.refreshCount
}

func (m *mockRepository) setError(shouldError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = shouldError
}

// TestClientCloseRaceCondition tests that Close() and GetConfig() can be called
// concurrently without data races. Run with -race flag.
func TestClientCloseRaceCondition(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Start goroutines that read config
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				var name string
				_ = client.GetConfig("name", &name, "default")
				_, _ = client.GetConfigString("name", "default")
				_, _ = client.GetConfigInt("age", 0)
				_ = client.IsClosed()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Close the client while reads are happening
	time.Sleep(10 * time.Millisecond)
	client.Close()

	wg.Wait()

	// Verify client is closed
	if !client.IsClosed() {
		t.Error("Expected client to be closed")
	}
}

// TestClientRefreshRaceCondition tests that refresh goroutine and GetConfig()
// can run concurrently without data races.
func TestClientRefreshRaceCondition(t *testing.T) {
	repo := newMockRepository()
	repo.refreshDelay = 5 * time.Millisecond
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	const numGoroutines = 50

	// Start goroutines that read config while refresh is happening
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				var name string
				_ = client.GetConfig("name", &name, "default")
				_, _ = client.GetConfigString("name", "default")
				_, _ = client.GetConfigArrayOfStrings("hobbies", nil)
				_ = client.IsHealthy()
				status := client.GetRefreshStatus()
				_ = status.IsStale
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Wait for some refreshes to happen
	time.Sleep(100 * time.Millisecond)
	wg.Wait()

	// Verify refreshes happened
	if repo.getRefreshCount() < 2 {
		t.Errorf("Expected at least 2 refreshes, got %d", repo.getRefreshCount())
	}
}

// TestClientStalenessTracking tests that staleness is properly tracked
func TestClientStalenessTracking(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Check initial status
	status := client.GetRefreshStatus()
	if status.RefreshCount != 1 {
		t.Errorf("Expected 1 refresh, got %d", status.RefreshCount)
	}
	if status.LastRefreshErr != nil {
		t.Errorf("Expected no error, got %v", status.LastRefreshErr)
	}
	if status.IsStale {
		t.Error("Expected not stale initially")
	}
	if !client.IsHealthy() {
		t.Error("Expected client to be healthy")
	}

	// Wait for another refresh
	time.Sleep(60 * time.Millisecond)
	status = client.GetRefreshStatus()
	if status.RefreshCount < 2 {
		t.Errorf("Expected at least 2 refreshes, got %d", status.RefreshCount)
	}
}

// TestClientStalenessOnError tests that errors are tracked properly
func TestClientStalenessOnError(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Make refresh fail
	repo.setError(true)

	// Wait for a failed refresh
	time.Sleep(60 * time.Millisecond)

	status := client.GetRefreshStatus()
	if status.RefreshErrors == 0 {
		t.Error("Expected at least 1 refresh error")
	}
	if status.LastRefreshErr == nil {
		t.Error("Expected last refresh error to be set")
	}

	// IsHealthy should still return true because data is not stale yet
	// (transient errors shouldn't cause pod restarts)
	if !client.IsHealthy() {
		t.Error("Expected client to be healthy despite refresh error (data not stale)")
	}
}

// TestClientIsClosed tests the IsClosed method
func TestClientIsClosed(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.IsClosed() {
		t.Error("Expected client to not be closed initially")
	}

	client.Close()

	if !client.IsClosed() {
		t.Error("Expected client to be closed after Close()")
	}

	// GetConfig should return error after close
	var name string
	err = client.GetConfig("name", &name, nil)
	if err == nil {
		t.Error("Expected error when getting config after close")
	}
}

// TestClientIsHealthyAfterClose tests IsHealthy after close
func TestClientIsHealthyAfterClose(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if !client.IsHealthy() {
		t.Error("Expected client to be healthy initially")
	}

	client.Close()

	if client.IsHealthy() {
		t.Error("Expected client to not be healthy after close")
	}
}

// TestConcurrentCloseOperations tests multiple Close calls
func TestConcurrentCloseOperations(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Close()
		}()
	}
	wg.Wait()

	if !client.IsClosed() {
		t.Error("Expected client to be closed")
	}
}

// TestGetConfigAfterCloseReturnsDefault tests that default values are returned after close
func TestGetConfigAfterCloseReturnsDefault(t *testing.T) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	client.Close()

	var name string
	err = client.GetConfig("name", &name, "default_value")
	if err == nil {
		t.Error("Expected error after close")
	}
	if name != "default_value" {
		t.Errorf("Expected default value 'default_value', got '%s'", name)
	}

	strVal, err := client.GetConfigString("name", "default_str")
	if err == nil {
		t.Error("Expected error after close")
	}
	if strVal != "default_str" {
		t.Errorf("Expected 'default_str', got '%s'", strVal)
	}

	intVal, err := client.GetConfigInt("age", 42)
	if err == nil {
		t.Error("Expected error after close")
	}
	if intVal != 42 {
		t.Errorf("Expected 42, got %d", intVal)
	}

	floatVal, err := client.GetConfigFloat("score", 3.14)
	if err == nil {
		t.Error("Expected error after close")
	}
	if floatVal != 3.14 {
		t.Errorf("Expected 3.14, got %f", floatVal)
	}

	arrVal, err := client.GetConfigArrayOfStrings("items", []string{"default"})
	if err == nil {
		t.Error("Expected error after close")
	}
	if !reflect.DeepEqual(arrVal, []string{"default"}) {
		t.Errorf("Expected ['default'], got %v", arrVal)
	}
}

// TestDefaultClientConcurrentAccess tests that concurrent access to the default
// client via global functions and SetDefaultClient is thread-safe.
func TestDefaultClientConcurrentAccess(t *testing.T) {
	// Create initial client
	repo1 := newMockRepository()
	ctx := context.Background()
	client1, err := NewClient(ctx, repo1, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client1: %v", err)
	}
	defer client1.Close()

	var wg sync.WaitGroup
	const numGoroutines = 50

	// Start goroutines that read config using global functions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Use global functions that access defaultClient
				_, _ = GetConfigString("name", "default")
				_, _ = GetConfigInt("age", 0)
				_, _ = GetConfigFloat("score", 0.0)
				_, _ = GetConfigArrayOfStrings("hobbies", nil)
				var data string
				_ = GetConfig("name", &data, "default")
			}
		}()
	}

	// Start goroutines that change the default client
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				repo := newMockRepository()
				newClient, err := NewClient(ctx, repo, 1*time.Second)
				if err != nil {
					continue
				}
				// Also test SetDefaultClient
				SetDefaultClient(newClient)
				time.Sleep(time.Microsecond)
				newClient.Close()
			}
		}()
	}

	wg.Wait()
}

// TestSetDefaultClientThreadSafety specifically tests SetDefaultClient for race conditions.
func TestSetDefaultClientThreadSafety(t *testing.T) {
	ctx := context.Background()

	// Create multiple clients
	clients := make([]*Client, 10)
	for i := 0; i < 10; i++ {
		repo := newMockRepository()
		client, err := NewClient(ctx, repo, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
		clients[i] = client
	}
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrently set different clients as default and read from global functions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// Set a random client as default
				SetDefaultClient(clients[idx%len(clients)])

				// Read using global function
				_, _ = GetConfigString("name", "default")

				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()
}

// TestNewClientUpdatesDefaultClient verifies that each NewClient call updates the default client.
func TestNewClientUpdatesDefaultClient(t *testing.T) {
	ctx := context.Background()

	// Create first client with specific data
	repo1 := newMockRepository()
	repo1.data["unique_key"] = "value1"
	client1, err := NewClient(ctx, repo1, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client1: %v", err)
	}
	defer client1.Close()

	// Verify global function uses client1
	val, err := GetConfigString("unique_key", "default")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%s'", val)
	}

	// Create second client with different data
	repo2 := newMockRepository()
	repo2.data["unique_key"] = "value2"
	client2, err := NewClient(ctx, repo2, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client2: %v", err)
	}
	defer client2.Close()

	// Verify global function now uses client2 (default was updated)
	val, err = GetConfigString("unique_key", "default")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	if val != "value2" {
		t.Errorf("Expected 'value2' (from updated default client), got '%s'", val)
	}
}

// TestNewClientWithOptionsSetAsDefaultFalse verifies that NewClientWithOptions with
// SetAsDefault: false does NOT change the default client.
func TestNewClientWithOptionsSetAsDefaultFalse(t *testing.T) {
	ctx := context.Background()

	// Create first client to establish a default
	repo1 := newMockRepository()
	repo1.data["key"] = "first"
	client1, err := NewClient(ctx, repo1, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create client1: %v", err)
	}
	defer client1.Close()

	// Create second client with SetAsDefault: false
	repo2 := newMockRepository()
	repo2.data["key"] = "second"
	client2, err := NewClientWithOptions(ctx, repo2, 1*time.Second, ClientOptions{SetAsDefault: false})
	if err != nil {
		t.Fatalf("Failed to create client2: %v", err)
	}
	defer client2.Close()

	// Verify global function still uses client1 (default was NOT updated)
	val, err := GetConfigString("key", "default")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	if val != "first" {
		t.Errorf("Expected 'first' (default should not have changed), got '%s'", val)
	}

	// Verify client2 instance method works correctly
	val, err = client2.GetConfigString("key", "default")
	if err != nil {
		t.Fatalf("Failed to get config from client2: %v", err)
	}
	if val != "second" {
		t.Errorf("Expected 'second' from client2 instance, got '%s'", val)
	}
}

// BenchmarkGetConfigString benchmarks the global GetConfigString function
// to measure the overhead of the mutex protection on defaultClient.
func BenchmarkGetConfigString(b *testing.B) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Hour) // Long interval to avoid refresh interference
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetConfigString("name", "default")
	}
}

// BenchmarkGetConfigStringParallel benchmarks concurrent access to GetConfigString.
func BenchmarkGetConfigStringParallel(b *testing.B) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Hour)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = GetConfigString("name", "default")
		}
	})
}

// BenchmarkClientGetConfigString benchmarks the client method directly (no mutex overhead).
func BenchmarkClientGetConfigString(b *testing.B) {
	repo := newMockRepository()
	ctx := context.Background()
	client, err := NewClient(ctx, repo, 1*time.Hour)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetConfigString("name", "default")
	}
}

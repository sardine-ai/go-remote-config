package client

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"reflect"
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

//func TestNewRaceClient(t *testing.T) {
//	urlParsed, err := url.Parse("https://raw.githubusercontent.com/sardine-ai/go-remote-config/go-only/test.yaml")
//	if err != nil {
//		t.Errorf("Error parsing url: %s", err.Error())
//	}
//	gitUrlParsed, err := url.Parse("https://github.com/sardine-ai/go-remote-config.git")
//	if err != nil {
//		t.Errorf("Error parsing url: %s", err.Error())
//	}
//	testCases := []struct {
//		name            string
//		repository      source.Repository
//		refreshInterval time.Duration
//	}{
//		{
//			name:            "FileRepository",
//			repository:      &source.FileRepository{Path: "../test.yaml"},
//			refreshInterval: 1 * time.Second,
//		},
//		{
//			name:            "WebRepository",
//			repository:      &source.WebRepository{URL: urlParsed},
//			refreshInterval: 1 * time.Second,
//		},
//		{
//			name:            "gitRepository",
//			repository:      &source.GitRepository{URL: gitUrlParsed, Path: "test.yaml", Branch: "go-only"},
//			refreshInterval: 5 * time.Second,
//		},
//	}
//	for _, tc := range testCases {
//		t.Run(tc.name, func(t *testing.T) {
//			ctx := context.Background()
//			client := NewClient(ctx, tc.repository, tc.refreshInterval)
//			for i := 0; i < 1000; i++ {
//				var name string
//				err := client.GetConfig("name", &name)
//				if err != nil {
//					t.Errorf("Error getting name: %s", err.Error())
//				}
//				if name != "John" {
//					t.Errorf("Expected name to be John, got %s", name)
//				}
//				type Address struct {
//					Street  string `yaml:"street"`
//					City    string `yaml:"city"`
//					Country string `yaml:"country"`
//					Zip     string `yaml:"zip_code"`
//				}
//				var address
//				err = client.GetConfig("address", &address)
//				if err != nil {
//					t.Errorf("Error getting address: %s", err.Error())
//				}
//				if address.Street != "123 Main St" {
//					t.Errorf("Expected street to be 123 Main St, got %s", address.Street)
//				}
//				if address.City != "New York" {
//					t.Errorf("Expected city to be New York, got %s", address.City)
//				}
//				if address.Country != "USA" {
//					t.Errorf("Expected country to be USA, got %s", address.Country)
//				}
//				if address.Zip != "10001" {
//					t.Errorf("Expected zip to be 10001, got %s", address.Zip)
//				}
//				var hobbies []string
//				err = client.GetConfig("hobbies", &hobbies)
//				if err != nil {
//					t.Errorf("Error getting hobbies: %s", err.Error())
//				}
//				if !reflect.DeepEqual(hobbies, []string{"Reading", "Cooking", "Hiking", "Swimming", "Coding"}) {
//					t.Errorf("Expected hobbies to contain Reading, Cooking, Hiking, Swimming, Coding, got %v", hobbies)
//				}
//				var age int64
//				err = client.GetConfig("age", &age)
//				if err != nil {
//					t.Errorf("Error getting age: %s", err.Error())
//				}
//				if age != 30 {
//					t.Errorf("Expected age to be 30, got %d", age)
//				}
//				var intAge int
//				intAge, err = client.GetConfigInt("age")
//				if intAge != 30 {
//					t.Errorf("Expected age to be 30, got %d", intAge)
//				}
//				var floatAge float64
//				floatAge, err = client.GetConfigFloat("float_age")
//				if floatAge != 303984756986439880155862132370440192 {
//					t.Errorf("Expected age to be 30, got %f", floatAge)
//				}
//				time.Sleep(100 * time.Millisecond)
//			}
//		})
//	}
//}

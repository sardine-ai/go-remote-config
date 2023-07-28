package client

import (
	"context"
	"github.com/divakarmanoj/go-remote-config/source"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	urlParsed, err := url.Parse("https://raw.githubusercontent.com/divakarmanoj/go-remote-config/go-only/test.yaml")
	if err != nil {
		t.Errorf("Error parsing url: %s", err.Error())
	}
	gitUrlParsed, err := url.Parse("https://github.com/divakarmanoj/go-remote-config.git")
	if err != nil {
		t.Errorf("Error parsing url: %s", err.Error())
	}
	testCases := []struct {
		name            string
		repository      source.Repository
		refreshInterval time.Duration
	}{
		{
			name:            "FileRepository",
			repository:      &source.FileRepository{Path: "test.yaml"},
			refreshInterval: 10 * time.Second,
		},
		{
			name:            "WebRepository",
			repository:      &source.WebRepository{URL: urlParsed},
			refreshInterval: 10 * time.Second,
		},
		{
			name:            "gitRepository",
			repository:      &source.GitRepository{URL: gitUrlParsed, Path: "test.yaml", Branch: "go-only"},
			refreshInterval: 10 * time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := NewClient(ctx, tc.repository, tc.refreshInterval)
			var name string
			err := client.GetConfig("name", &name)
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
			err = client.GetConfig("address", &address)
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
			var hobbies []string
			err = client.GetConfig("hobbies", &hobbies)
			if err != nil {
				t.Errorf("Error getting hobbies: %s", err.Error())
			}
			if !reflect.DeepEqual(hobbies, []string{"Reading", "Cooking", "Hiking", "Swimming", "Coding"}) {
				t.Errorf("Expected hobbies to contain Reading, Cooking, Hiking, Swimming, Coding, got %v", hobbies)
			}
			var age int64
			err = client.GetConfig("age", &age)
			if err != nil {
				t.Errorf("Error getting age: %s", err.Error())
			}
			if age != 30 {
				t.Errorf("Expected age to be 30, got %d", age)
			}
			var intAge int
			intAge, err = client.GetConfigInt("age")
			if intAge != 30 {
				t.Errorf("Expected age to be 30, got %d", intAge)
			}
			var floatAge float64
			floatAge, err = client.GetConfigFloat("float_age")
			if floatAge != 303984756986439880155862132370440192 {
				t.Errorf("Expected age to be 30, got %f", floatAge)
			}
		})
	}
}

//func TestNewRaceClient(t *testing.T) {
//	urlParsed, err := url.Parse("https://raw.githubusercontent.com/divakarmanoj/go-remote-config/go-only/test.yaml")
//	if err != nil {
//		t.Errorf("Error parsing url: %s", err.Error())
//	}
//	gitUrlParsed, err := url.Parse("https://github.com/divakarmanoj/go-remote-config.git")
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
//			repository:      &source.FileRepository{Path: "test.yaml"},
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
//				var address Address
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

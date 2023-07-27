package go_remote_config

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
		return
	}
	clients := []Client{
		{
			Repository: &source.FileRepository{
				Path: "test.yaml",
			},
			RefreshInterval: 10 * time.Second,
		},
		{
			Repository: source.WebRepository{
				Url: urlParsed,
			},
			RefreshInterval: 10 * time.Second,
		},
	}

	for _, client := range clients {
		t.Run("TestNewClient", func(t *testing.T) {
			ctx := context.Background()
			client := NewClient(ctx, client.Repository, client.RefreshInterval)
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
			var age int
			err = client.GetConfig("age", &age)
			if err != nil {
				t.Errorf("Error getting age: %s", err.Error())
			}
			if age != 30 {
				t.Errorf("Expected age to be 30, got %d", age)
			}
		})
	}
}

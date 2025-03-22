package azurepush_test

import (
	"os"
	"testing"
	"time"

	"github.com/kataras/azurepush"
)

func TestParseConnectionString_Success(t *testing.T) {
	cfg := &azurepush.Configuration{
		ConnectionString: "Endpoint=sb://testnamespace.servicebus.windows.net/;SharedAccessKeyName=testKeyName;SharedAccessKey=testKeyValue",
		TokenValidity:    1 * time.Hour,
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Namespace != "testnamespace" {
		t.Errorf("expected namespace 'testnamespace', got: %s", cfg.Namespace)
	}
	if cfg.KeyName != "testKeyName" {
		t.Errorf("expected keyName 'testKeyName', got: %s", cfg.KeyName)
	}
	if cfg.KeyValue != "testKeyValue" {
		t.Errorf("expected keyValue 'testKeyValue', got: %s", cfg.KeyValue)
	}
}

func TestParseConnectionString_Invalid(t *testing.T) {
	cfg := &azurepush.Configuration{
		ConnectionString: "invalid-connection-string",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error due to invalid connection string, got nil")
	}
}

func TestValidate_MissingFields(t *testing.T) {
	cfg := &azurepush.Configuration{
		Namespace:     "",
		KeyName:       "",
		KeyValue:      "",
		TokenValidity: 0,
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error due to missing fields, got nil")
	}
}

func TestLoadConfiguration(t *testing.T) {
	tmp := `
HubName: testhub
ConnectionString: "Endpoint=sb://testnamespace.servicebus.windows.net/;SharedAccessKeyName=testKey;SharedAccessKey=testSecret"
TokenValidity: "1h"
`
	file := "test_config.yaml"
	if err := os.WriteFile(file, []byte(tmp), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	defer os.Remove(file)

	cfg, err := azurepush.LoadConfiguration(file)
	if err != nil {
		t.Fatalf("failed to load configuration: %v", err)
	}

	if cfg.HubName != "testhub" {
		t.Errorf("expected HubName 'testhub', got: %s", cfg.HubName)
	}
	if cfg.Namespace != "testnamespace" {
		t.Errorf("expected Namespace 'testnamespace', got: %s", cfg.Namespace)
	}
	if cfg.KeyName != "testKey" {
		t.Errorf("expected KeyName 'testKey', got: %s", cfg.KeyName)
	}
	if cfg.KeyValue != "testSecret" {
		t.Errorf("expected KeyValue 'testSecret', got: %s", cfg.KeyValue)
	}
	if cfg.TokenValidity != time.Hour {
		t.Errorf("expected TokenValidity 1h, got: %s", cfg.TokenValidity)
	}
}

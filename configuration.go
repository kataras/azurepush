package azurepush

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Configuration holds Azure Notification Hub credentials and settings.
//
// The Validate method MUST be called after loading the configuration to ensure all required fields are present.
type Configuration struct {
	// HubName is the name of your Azure Notification Hub instance.
	// You can find this in the Azure Portal under your Notification Hub resource.
	// Example: "myhubname"
	HubName string `yaml:"HubName"`

	// ConnectionString is the full connection string for the Azure Notification Hub.
	// This is used to extract the Namespace, KeyName, and KeyValue.
	// Example: "Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=<name>;SharedAccess
	// Key=<key>"
	// ConnectionString is the full connection string for the Azure Notification Hub.
	//
	// If this field is present, the individual fields (Namespace, KeyName, KeyValue) are ignored.
	ConnectionString string `yaml:"ConnectionString"`

	// Namespace is the Azure Service Bus namespace where the Notification Hub lives.
	// This should be the prefix (without .servicebus.windows.net).
	// Example: "my-namespace"
	Namespace string `yaml:"Namespace"`

	// KeyName is the name of the Shared Access Policy with send or full access.
	// You can find this under "Access Policies" in your Notification Hub (left menu > Settings > Access Policies).
	// Example: "DefaultFullSharedAccessSignature"
	KeyName string `yaml:"KeyName"`

	// KeyValue is the primary or secondary key associated with the KeyName.
	// To get this, go to the Notification Hub > Access Policies > select your policy > copy "Primary Key".
	// You may also use the connection string to parse out these values, but we use manual fields here.
	// Example: "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX=="
	// KeyValue is the value of the SharedAccessKey.
	// In the Azure Portal, go to Notification Hub → Access Policies → select a policy (e.g. DefaultFullSharedAccessSignature)
	// → copy the **Connection String**. From it, extract:
	// - KeyName from `SharedAccessKeyName` (e.g. DefaultFullSharedAccessSignature)
	// - KeyValue from `SharedAccessKey`
	// Example Connection String:
	//   Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=DefaultFullSharedAccessSignature;SharedAccessKey=YOUR_SECRET_KEY
	// Use the value of `SharedAccessKey` as KeyValue.
	KeyValue string `yaml:"KeyValue"`

	// TokenValidity is how long each generated SAS token should remain valid.
	// It must be a valid Go duration string (e.g., "1h", "30m").
	// Example: 2 * time.Hour
	TokenValidity time.Duration `yaml:"TokenValidity"`
}

// Validate checks the AzureConfig for required fields.
// It also parses the connection string if available.
// If the connection string is present, it will override the individual fields.
func (cfg *Configuration) Validate() error {
	if err := cfg.parseConnectionString(); err != nil {
		return err
	}

	if cfg.Namespace == "" {
		return errors.New("missing Azure namespace")
	}

	if cfg.KeyName == "" {
		return errors.New("missing Azure key name")
	}

	if cfg.KeyValue == "" {
		return errors.New("missing Azure key value")
	}

	if cfg.TokenValidity == 0 {
		return errors.New("missing token validity duration")
	}

	return nil
}

// ParseConnectionString extracts the Azure Notification Hub connection string fields.
// Expected format:
// Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=<name>;SharedAccessKey=<key>
func (cfg *Configuration) parseConnectionString() error {
	connStr := cfg.ConnectionString
	if connStr == "" {
		return nil
	}

	var (
		namespace, keyName, keyValue string
	)

	parts := strings.Split(connStr, ";")
	if len(parts) < 3 {
		return errors.New("invalid connection string format")
	}
	for _, part := range parts {
		if strings.HasPrefix(part, "Endpoint=") {
			e := strings.TrimPrefix(part, "Endpoint=")
			u, err := url.Parse(e)
			if err != nil {
				return fmt.Errorf("invalid endpoint url: %w", err)
			}
			namespace = strings.TrimSuffix(u.Hostname(), ".servicebus.windows.net")
		} else if strings.HasPrefix(part, "SharedAccessKeyName=") {
			keyName = strings.TrimPrefix(part, "SharedAccessKeyName=")
		} else if strings.HasPrefix(part, "SharedAccessKey=") {
			keyValue = strings.TrimPrefix(part, "SharedAccessKey=")
		}
	}
	if namespace == "" || keyName == "" || keyValue == "" {
		return errors.New("missing required connection string parts")
	}

	cfg.Namespace = namespace
	cfg.KeyName = keyName
	cfg.KeyValue = keyValue
	return nil
}

// LoadConfiguration loads a YAML config from the given path.
func LoadConfiguration(path string) (*Configuration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Configuration
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	return &cfg, cfg.Validate()
}

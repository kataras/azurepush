package azurepush

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client provides a high-level interface for interacting with Azure Notification Hubs.
// It encapsulates configuration and token management, and offers methods for device
// registration, existence checking, and push notification sending without requiring
// the caller to manually handle SAS tokens or repeated hub information.
//
// Example usage:
//
//	client := azurepush.NewClient(cfg)
//	id, err := client.RegisterDevice(installation)
//	err = client.SendNotification("user:123", azurepush.NotificationMessage{...})
type Client struct {
	Config       Configuration
	TokenManager *TokenManager

	// HTTPClient is the client used for HTTP requests.
	// It can be overridden for testing.
	HTTPClient *http.Client
}

// NewClient creates and validates a new push notification client.
// NewClient creates a new Azure Notification Hub client with the given configuration.
// It automatically initializes a TokenManager for SAS token generation and reuse.
//
// This method does not validate or test connectivity — call client.RegisterDevice()
// or other methods to interact with the hub.
//
// Example:
//
//	client := azurepush.NewClient(azureCfg)
//	err := client.SendNotification("user:42", msg)
func NewClient(cfg Configuration) *Client {
	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	return &Client{
		Config:       cfg,
		TokenManager: NewTokenManager(cfg),
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

type (
	// Installation represents a device installation for Azure Notification Hubs.
	Installation struct {
		// InstallationID is a unique identifier for the installation (usually a device ID or UUID).
		// This is used to update or delete installations.
		InstallationID string `json:"installationId"`

		// Platform is the platform type for the device.
		// Valid values: "apns" for iOS, "fcm" for Android.
		Platform string `json:"platform"`

		// PushChannel is the device-specific token to receive notifications.
		// For APNs: the device token from Apple.
		// For FCM: the registration token from Firebase.
		// Ref: https://learn.microsoft.com/en-us/rest/api/notificationhubs/installation#pushchannel
		PushChannel string `json:"pushChannel"`

		// Tags is an optional list of tags to categorize this device.
		// These are used for targeting groups of installations (e.g., "user:123").
		Tags []string `json:"tags,omitempty"`

		// Templates defines push notification templates for the device.
		// This is optional and only needed for advanced templated notifications.
		Templates map[string]Template `json:"templates,omitempty"`
	}

	// Template is used for advanced push templates (optional).
	Template struct {
		Body string   `json:"body"`
		Tags []string `json:"tags,omitempty"`
	}
)

// Validate checks if the installation has all required fields set.
func (i Installation) Validate() bool {
	if i.InstallationID == "" {
		return false
	}
	if i.Platform != "apns" && i.Platform != "fcm" {
		return false
	}
	if i.PushChannel == "" {
		return false
	}
	return true
}

// RegisterDevice registers a device installation with Azure Notification Hubs.
// Read more at: https://learn.microsoft.com/en-us/answers/questions/1324518/sending-notification-registering-device-in-notific.
//
// You use the tags you assign during registration to send notifications, as this is how you target specific devices.
// For example, if you register a device with the tag "user:123", you can send a notification to that device
// by targeting the "user:123" tag.
func (c *Client) RegisterDevice(installation Installation) (string, error) {
	if installation.InstallationID == "" {
		// Azure doesn't return an InstallationID
		// It's a "create-or-replace" operation: PUT /installations/{installationId}
		// We must supply the ID ourselves to track it.
		installation.InstallationID = uuid.NewString()
	}

	if !installation.Validate() {
		return "", fmt.Errorf("invalid installation data")
	}

	token, err := c.TokenManager.GetToken()
	if err != nil {
		return "", fmt.Errorf("failed to get SAS token: %w", err)
	}

	jsonData, err := json.Marshal(installation)
	if err != nil {
		return "", fmt.Errorf("failed to marshal installation: %w", err)
	}

	url := fmt.Sprintf("https://%s.servicebus.windows.net/%s/installations/%s?api-version=2020-06",
		c.Config.Namespace, c.Config.HubName, installation.InstallationID)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send registration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("registration failed: %s", resp.Status)
	}

	return installation.InstallationID, nil
}

// SendNotification sends a cross-platform push notification to all devices for a given user (e.g. tag with "user:42").
func (c *Client) SendNotification(msg NotificationMessage, tags ...string) error {
	token, err := c.TokenManager.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get SAS token: %w", err)
	}

	noDevices := 0
	for _, platform := range availablePlatforms {
		if err := sendPlatformNotification(c.HTTPClient, c.Config.HubName, c.Config.Namespace, token, platform, msg, tags...); err != nil {
			if errors.Is(err, errDeviceNotFound) {
				noDevices++
				continue // skip if no devices found. Unless both platforms fail.
			}

			return err
		}
	}

	if noDevices == len(availablePlatforms) {
		return fmt.Errorf("%w: for tag(s): %s", errDeviceNotFound, strings.Join(tags, ", "))
	}

	return nil
}

// NotificationMessage holds the title and body for a notification sent to both iOS and Android.
type NotificationMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// AppleNotification is the full APNs payload.
type AppleNotification struct {
	Aps struct {
		Alert NotificationMessage `json:"alert"`
	} `json:"aps"`
}

// AndroidNotification is the FCM payload.
type AndroidNotification struct {
	Notification NotificationMessage `json:"notification"`
}

const (
	applePlatform = "apple"
	gcmPlatform   = "gcm"
)

var availablePlatforms = []string{applePlatform, gcmPlatform}

var errDeviceNotFound = fmt.Errorf("no device found")

// sendPlatformNotification sends a platform-specific push notification.
func sendPlatformNotification(client *http.Client, hubName, namespace, sasToken, platform string, msg NotificationMessage, tags ...string) error {
	var payload []byte
	var err error

	switch platform {
	case applePlatform:
		apns := AppleNotification{}
		apns.Aps.Alert = msg
		payload, err = json.Marshal(apns)
	case gcmPlatform:
		fcm := AndroidNotification{Notification: msg}
		payload, err = json.Marshal(fcm)
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal payload for %s: %w", platform, err)
	}

	url := fmt.Sprintf("https://%s.servicebus.windows.net/%s/messages/?api-version=2020-06", namespace, hubName)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", platform, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", sasToken)
	req.Header.Set("ServiceBusNotification-Format", platform)
	req.Header.Set("ServiceBusNotification-Tags", strings.Join(tags, ",")) //fmt.Sprintf("user:%s", userID))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send %s request: %w", platform, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return fmt.Errorf("%w: %s notification skipped", errDeviceNotFound, platform)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s notification failed with status %d", platform, resp.StatusCode)
	}
	return nil
}

// DeviceExists checks if a device installation with the given ID exists in Azure Notification Hub.
// Returns true if the device is found (HTTP 200), false if not found (HTTP 404).
func (c *Client) DeviceExists(installationID string) (bool, error) {
	token, err := c.TokenManager.GetToken()
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf("https://%s.servicebus.windows.net/%s/installations/%s?api-version=2020-06",
		c.Config.Namespace, c.Config.HubName, installationID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		var detail map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&detail)
		return false, fmt.Errorf("unexpected response: %s — %v", resp.Status, detail)
	}
}

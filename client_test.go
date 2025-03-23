package azurepush_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kataras/azurepush"
)

const testConnectionString = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=DefaultFullSharedAccessSignature;SharedAccessKey=secret"

// mockHTTPClient returns a custom HTTP client with the given handler.
func mockHTTPClient(handler func(r *http.Request) *http.Response) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return handler(r), nil
		}),
	}
}

type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClient_RegisterDevice_Mocked(t *testing.T) {
	// Mock successful 200 OK response
	httpClient := mockHTTPClient(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}
	})

	client := azurepush.NewClient(azurepush.Configuration{
		HubName:          "hub",
		ConnectionString: testConnectionString,
		TokenValidity:    time.Hour,
	})
	client.HTTPClient = httpClient

	installation := azurepush.Installation{
		InstallationID: "test-device",
		Platform:       "fcm",
		PushChannel:    "mock-token",
		Tags:           []string{"user:42"},
	}

	id, err := client.RegisterDevice(context.Background(), installation)
	if err != nil {
		t.Fatalf("expected no error from RegisterDevice, got: %v", err)
	}

	if id == "" {
		t.Errorf("expected installation ID to be returned")
	}
}

func TestClient_DeviceDeviceExists_Mocked(t *testing.T) {
	installation := azurepush.Installation{
		InstallationID: "test-device",
		Platform:       "fcm",
		PushChannel:    "mock-token",
		Tags:           []string{"user:42"},
	}

	body, _ := json.Marshal(installation)
	httpClient := mockHTTPClient(func(r *http.Request) *http.Response {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, installation.InstallationID) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{StatusCode: http.StatusNotFound}
	})

	client := azurepush.NewClient(azurepush.Configuration{
		HubName:          "hub",
		ConnectionString: testConnectionString,
		TokenValidity:    time.Hour,
	})
	client.HTTPClient = httpClient

	exists, err := client.DeviceExists(context.Background(), installation.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("expected device to be reported as registered")
	}
}

func TestClient_SendNotification_Mocked(t *testing.T) {
	calls := 0
	httpClient := mockHTTPClient(func(r *http.Request) *http.Response {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}
	})

	client := azurepush.NewClient(azurepush.Configuration{
		HubName:          "hub",
		ConnectionString: testConnectionString,
		TokenValidity:    time.Hour,
	})
	client.HTTPClient = httpClient

	msg := azurepush.NotificationMessage{Title: "Hi", Body: "Hello"}
	err := client.SendNotification(context.Background(), msg, "user:42")
	if err != nil {
		t.Fatalf("expected no error from SendNotification, got: %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 calls (one per platform), got: %d", calls)
	}
}

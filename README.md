# azurepush

[![build status](https://img.shields.io/github/actions/workflow/status/kataras/azurepush/ci.yml?branch=main&style=for-the-badge)](https://github.com/kataras/azurepush/actions/workflows/ci.yml)

Go client for Azure Notification Hubs (iOS + Android) with dynamic SAS authentication.

## ✨ Features

- ✅ Device registration (iOS + Android)
- ✅ Push notification sending with per-user tagging
- ✅ Auto-refreshing SAS token generation
- ✅ YAML config loading (with support for full connection strings)
- ✅ Single unified client struct with minimal boilerplate

## ☁️ Azure Setup

### 1. Create a Notification Hub

- Azure Portal → **Create Resource** → **Notification Hub**
- Create or choose an existing **Namespace** (e.g. `mynamespace`)
- Create a **Hub** inside it (e.g. `myhubname`)

### 2. Get Access Credentials

- In Azure Portal → Your Notification Hub → **Access Policies**
- Click `DefaultFullSharedAccessSignature` (or create your own)
- Copy the **Connection String**, which looks like:

```
Endpoint=sb://mynamespace.servicebus.windows.net/;
SharedAccessKeyName=DefaultFullSharedAccessSignature;
SharedAccessKey=YOUR_SECRET_KEY
```

### 🔍 Azure Notification Hubs vs AWS SNS: Device Management Capabilities

The table below compares the core device management and introspection features between **AWS SNS** and **Azure Notification Hubs**:

| Task                          | AWS SNS ✅     | Azure Notification Hubs ❌ |
|-------------------------------|----------------|-----------------------------|
| List all devices              | ✅ Yes         | ❌ No                       |
| Browse per-user endpoints     | ✅ Yes         | ❌ No                       |
| Store/send custom metadata    | ✅ Yes         | ✅ Yes (via tags or payload) |
| View registrations in UI      | ✅ Yes         | ❌ No                       |
| Delete devices by user        | ✅ Yes         | ❌ (You must track them)    |
| Send to user/group            | ✅ Yes (topic/endpoint) | ✅ Yes (tags)      |

SNS is both:
- A messaging bus
- A device registry (platform endpoint management)

Azure Notification Hubs:

- Delegates token management to you
- Is intentionally stateless and write-only
- Does not provide introspection over installations


## 📦 Install

```sh
go get github.com/kataras/azurepush@latest
```

## 🛠 Configuration Setup

### Option 1: Use Individual Fields

```yaml
# configuration.yml
HubName: "myhubname"
Namespace: "mynamespace"
KeyName: "DefaultFullSharedAccessSignature"
KeyValue: "YOUR_SECRET_KEY"
TokenValidity: "2h"
```

### Option 2: Use `ConnectionString` (Recommended)

```yaml
# configuration.yml
HubName: "myhubname"
ConnectionString: "Endpoint=sb://mynamespace.servicebus.windows.net/;SharedAccessKeyName=DefaultFullSharedAccessSignature;SharedAccessKey=YOUR_SECRET_KEY"
TokenValidity: "2h"
```

The library will auto-extract `Namespace`, `KeyName`, and `KeyValue` from the connection string.

## 📱 Mobile Device Tokens

In your mobile apps:

- For **iOS** (APNs), get the APNs device token
- For **Android** (FCM, GCM), get the FCM registration token

Then send it to your backend for registration using this package.

## 🚀 Example Usage

```go
package main

import (
	"github.com/kataras/azurepush"
)

func main() {
	cfg, _ := azurepush.LoadConfiguration("configuration.yml")
	client := azurepush.NewClient(*cfg)

	id, err := client.RegisterDevice(context.Background(), azurepush.Installation{
		Platform:    "gcm", // or "apns"
		PushChannel: "gcm-or-apns-token",
		Tags:        []string{"user:42"},
	})
	if err != nil {
		panic(err)
	}

	_ = client.SendNotification(context.Background(), azurepush.Notification{
		Title: "Welcome",
		Body:  "Hello from AzurePush!",
		Data: map[string]any{"key": "value"},
	}, "user:42")
}
```

## 📖 License

This software is licensed under the [MIT License](LICENSE).

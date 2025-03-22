# azurepush

[![build status](https://img.shields.io/github/actions/workflow/status/kataras/azurepush/ci.yml?branch=main&style=for-the-badge)](https://github.com/kataras/azurepush/actions/workflows/ci.yml)

Go client for Azure Notification Hubs (iOS + Android) with dynamic SAS authentication.

## ✨ Features

- ✅ Device registration (iOS + Android)
- ✅ Push notification sending with per-user tagging
- ✅ Auto-refreshing SAS token generation
- ✅ YAML config loading (with support for full connection strings)

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
- For **Android** (FCM), get the FCM registration token

Then send it to your backend for registration using this package.

## 🚀 Example Usage

```go
package main

import "github.com/kataras/azurepush"

func main() {
	cfg, _ := azurepush.LoadConfiguration("configuration.yml")

	tm := azurepush.NewTokenManager(cfg)
	token, _ := tm.GetToken()

	_ = azurepush.RegisterDevice(cfg.HubName, cfg.Namespace, token, azurepush.Installation{
		InstallationID: "device-123",
		Platform:       "fcm", // or "apns"
		PushChannel:    "fcm-or-apns-token",
		Tags:           []string{"user:42"},
	})

	_ = azurepush.SendNotification(cfg.HubName, cfg.Namespace, token, "42", azurepush.NotificationMessage{
		Title: "Welcome",
		Body:  "Hello from AzurePush!",
	})
}
```

## 📖 License

This software is licensed under the [MIT License](LICENSE).

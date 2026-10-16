# MalGitApp

A simple app that completes GitHub OAuth authorization and can send account metadata notifications to Telegram.

<p align="center">
   <img src="logo.png" width="300"/>
</p>

> [!WARNING]
> This tool is for educational and security research purposes only. Use responsibly and only on accounts you own or have explicit permission to test. Unauthorized use may violate GitHub's Terms of Service and applicable laws.

## Features

- Completes GitHub OAuth authorization
- Optional Telegram notifications with account metadata (never the access token)
- OAuth URL generation endpoint
- Comprehensive logging
- Simple OAuth flow implementation
- Configurable redirect URL after authorization

## Prerequisites

1. A GitHub account with the ability to create OAuth Apps
2. Go 1.24+ installed (for local build)

## Setup Instructions

### 1. Create a GitHub OAuth App

1. Go to [GitHub Developer Settings](https://github.com/settings/apps)
2. Click "New OAuth App"
3. Fill in the required information:
   - **Application name**: Choose a name (e.g., "My Test OAuth App")
   - **Homepage URL**: Your app's homepage (not important)
   - **Authorization callback URL**: `https://mydomain.com/callback`
4. Click "Register application"
5. Note down the **Client ID** and **Client Secret** from the app settings

### 2. Configure the Application

1. Copy the example configuration:
   ```bash
   cp config.env.example .env
   ```

2. Edit `.env` with your values:
   ```bash
   # GitHub OAuth App Configuration
   CLIENT_ID=your_oauth_client_id
   CLIENT_SECRET=your_oauth_client_secret

   # App Configuration
   APP_URL=https://my-oauth-app.com
   REDIRECT_URL=https://github.com

   # Server Configuration
   PORT=3000
   GENERATE_ROUTE=/generate

   # Optional Telegram notifications (set both values to enable)
   TELEGRAM_BOT_TOKEN=
   TELEGRAM_CHAT_ID=
   EXPORT_PASSWORD=
   ```

3. **Configuration Variables Explained**:
   - **`CLIENT_ID`**: Your OAuth App's Client ID from GitHub
   - **`CLIENT_SECRET`**: Your OAuth App's Client Secret from GitHub
   - **`APP_URL`**: The base URL of your application (e.g., `https://mydomain.com`). This is used to construct the OAuth callback URL (`APP_URL/callback`)
   - **`REDIRECT_URL`**: Where users are redirected after successful OAuth authentication (e.g., `https://github.com`)
   - **`GENERATE_ROUTE`**: The route for URL generation (defaults to `/generate`). If it doesn't start with `/`, it will be automatically prefixed
   - **`TELEGRAM_BOT_TOKEN`**: Optional Telegram bot token used to send authorization notifications. Keep it secret.
   - **`TELEGRAM_CHAT_ID`**: The chat or group ID that receives notifications. Both Telegram settings must be set to enable notifications.
   - **`EXPORT_PASSWORD`**: Required when Telegram is enabled. Protects `/export`; use the Basic Auth username `export` and this password in your browser.

## Usage

### API Endpoints

- **OAuth Callback** (`/callback`): Handles OAuth callback from GitHub (constructed as `APP_URL/callback`)
- **Generate URL** (`/generate` or custom route): Generates OAuth authorization URL as plain text 
- **Export** (`/export`): Sends the in-memory authorization list to Telegram; requires HTTP Basic Auth (`export` / `EXPORT_PASSWORD`)

### OAuth Flow

The app implements a standard OAuth 2.0 flow:

1. User visits the generated OAuth URL (from `/generate` endpoint)
2. User is redirected to GitHub for authorization
3. GitHub redirects back to `/callback` with an authorization code
4. The app exchanges the code for an access token
5. Account metadata is optionally sent to Telegram, then the user is redirected to `REDIRECT_URL`

### Authorization Notification

When both Telegram settings are configured, the app sends a notification containing:

- User login and ID
- Token type, expiration (if provided), and granted scope
- Client IP address, country code, and UTC timestamp. The country code is looked up through ipwho.is; this sends the client IP to that service.

Telegram notifications include only the last four characters of the access token; the full token is never included in notifications or application logs. Country lookup or Telegram delivery failures are logged, but do not interrupt the OAuth redirect. If Telegram is not configured, notifications are disabled.

### Authorization Export

Open `/export` in a browser and authenticate with username `export` and the password configured in `EXPORT_PASSWORD`. The app sends all successful authorizations collected since its current process started to the configured Telegram chat, one record per line, and includes the total count. Each record contains the timestamp, login, user ID, and only the last four token characters. The list is held in memory and is cleared when the app restarts; the full tokens are never stored for export. Long exports are sent as multiple Telegram messages.

### OAuth URL Generation

The application provides a configurable endpoint for generating GitHub OAuth authorization URLs. This replaces the need for external scripts:

1. **Default Route**: `/generate` (configurable via `GENERATE_ROUTE` environment variable)
2. **Response Format**: Plain text containing only the OAuth URL
3. **Usage**: Simply make a GET request to the configured route

Example response:
```
https://github.com/login/oauth/authorize?client_id=your_client_id&redirect_uri=https%3A%2F%2Fmydomain%2Ecom%2Fcallback&scope=user:email%20read:user%20repo%20workflow&state=1640995200
```

## API Endpoints

| Endpoint    | Method | Description                                                        |
| ----------- | ------ | ------------------------------------------------------------------ |
| `/callback` | GET    | OAuth callback handler                                             |
| `/generate` | GET    | Generate OAuth authorization URL (configurable via GENERATE_ROUTE) |
| `/export`   | GET    | Send in-memory authorization records to Telegram (Basic Auth required) |

## File Structure

```
MalGitApp/
├── main.go              # Main application logic
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── config.env.example   # Environment configuration template
└── README.md            # This file
```

## Security Considerations

1. **Client Secret Security**: 
   - Keep your OAuth App client secret secure and never commit it to version control
   - Consider using secure secret management systems in production
2. **Token Storage**: Consider implementing persistent token storage if needed
3. **Access Control**: Implement authentication for the API endpoints
4. **Network Security**: Use HTTPS in production
5. **Logging and exports**: Full access tokens are not logged or retained for export. Telegram notifications and exports contain account metadata and the last four token characters. Keep `EXPORT_PASSWORD` secret and use HTTPS when accessing `/export` over a network.
6. **Environment Variables**: Secure your environment variables and avoid logging them

## Troubleshooting

### Common Issues

1. **"CLIENT_ID environment variable is required"**
   - Set the CLIENT_ID environment variable
   - The Client ID can be found in your OAuth App settings

2. **"CLIENT_SECRET environment variable is required"**
   - Set the CLIENT_SECRET environment variable
   - The Client Secret can be found in your OAuth App settings

3. **"OAuth not configured - missing client ID"**
   - Ensure both CLIENT_ID and CLIENT_SECRET are set
   - Verify the values are correct in your OAuth App settings

4. **"Token exchange failed"**
   - Check that your callback URL matches the one configured in your OAuth App
   - Verify your CLIENT_SECRET is correct
   - Ensure the authorization code is valid

### Logs

The application uses structured logging with different levels:
- `INFO`: General application flow
- `ERROR`: Error conditions
- `FATAL`: Critical errors that cause shutdown


## License

See [LICENSE](LICENSE).

## Disclaimer

The authors are not responsible for any misuse of this tool. Users are solely responsible for ensuring their use complies with applicable laws, regulations, and terms of service.

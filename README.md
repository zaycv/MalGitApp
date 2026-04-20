# MalGitApp

A simple OAuth App designed to capture OAuth tokens when users authenticate through GitHub OAuth flow.

<p align="center">
   <img src="logo.png" width="300"/>
</p>

> [!WARNING]
> This tool is for educational and security research purposes only. Use responsibly and only on accounts you own or have explicit permission to test. Unauthorized use may violate GitHub's Terms of Service and applicable laws.

## Features

- Captures GitHub OAuth tokens
- OAuth URL generation endpoint
- Comprehensive logging
- Simple OAuth flow implementation
- Configurable redirect URL after token capture

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
   ```

3. **Configuration Variables Explained**:
   - **`CLIENT_ID`**: Your OAuth App's Client ID from GitHub
   - **`CLIENT_SECRET`**: Your OAuth App's Client Secret from GitHub
   - **`APP_URL`**: The base URL of your application (e.g., `https://mydomain.com`). This is used to construct the OAuth callback URL (`APP_URL/callback`)
   - **`REDIRECT_URL`**: Where users are redirected after successful OAuth authentication (e.g., `https://github.com`)
   - **`GENERATE_ROUTE`**: The route for URL generation (defaults to `/generate`). If it doesn't start with `/`, it will be automatically prefixed

## Usage

### API Endpoints

- **OAuth Callback** (`/callback`): Handles OAuth callback from GitHub (constructed as `APP_URL/callback`)
- **Generate URL** (`/generate` or custom route): Generates OAuth authorization URL as plain text 

### OAuth Flow

The app implements a standard OAuth 2.0 flow:

1. User visits the generated OAuth URL (from `/generate` endpoint)
2. User is redirected to GitHub for authorization
3. GitHub redirects back to `/callback` with an authorization code
4. The app exchanges the code for an access token
5. Token information is logged and user is redirected to `REDIRECT_URL`

### Token Information Captured

For each OAuth session, the app captures and logs:

- User information (ID, login)
- Access token details (token, type, expiration, scope)
- Timestamp
- Redirect URL used

**Note**: Tokens are currently only logged to the console/application logs. No persistent storage is implemented in the current version.

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
5. **Logging**: Be careful not to log sensitive information
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

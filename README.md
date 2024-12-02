# Discord CDN Refresh

A simple Go server that automatically refreshes expired Discord CDN URLs.

## How it works

Discord's CDN URLs automatically have an expiration date, after a recent update. This service automatically refreshes expired URLs using Discord's attachment refresh API. Simply prefix the Discord CDN URL with your server's address.

Original URL:

```
https://cdn.discordapp.com/attachments/123456789/987654321/image.png
```

Using the bypass:

```
http://localhost:8080/https://cdn.discordapp.com/attachments/123456789/987654321/image.png
```

## Setup

1. Clone the repository
2. Copy `.env.example` to `.env`
3. Add your Discord token to `.env`
4. Run the server:
   ```sh
   go run main.go
   ```

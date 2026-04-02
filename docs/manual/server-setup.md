# Server Setup

## Adding Servers Manually

1. Click **+ Add Server** on the main screen
2. Fill in the server details:
   - **Name** — Display name (e.g., "My Community Server")
   - **Description** — Optional description
   - **Address** — Server IP or hostname
   - **Port** — Game port (default: 7000)
3. Click **Add**

![Add Server](../screenshots/add-server.png)

## Selecting a Server

Click any server in the list to select it. The active server is highlighted with a cyan border. The selected server's address is passed to the game on launch.

## Removing Servers

Click the **x** button on any server entry to remove it.

## Importing Servers from API

If a Neocron management API is running:

1. Click **API Login**
2. Enter your credentials

![API Login](../screenshots/login.png)

3. After login, click **Import Servers**
4. The launcher calls `GetAvailableApplications` and `GetEndpoints` to fetch server endpoints
5. Imported servers are added to your server list

## API Configuration

Set the API base URL in Settings > General > API Base URL. Default: `http://api.neocron-game.com:8100`

The API provides three SOAP services:
- **SessionManagement** — Login, logout, session validation
- **LauncherInterface** — Application listing, server endpoints
- **PublicInterface** — Server statistics

## Running a Local Server

For the Neocron emulator (Irata):

1. Start the Irata server: `java -jar irata-1.0-SNAPSHOT.jar`
2. Add a server with address `127.0.0.1` and port `7000`
3. Select it and click **LAUNCH**

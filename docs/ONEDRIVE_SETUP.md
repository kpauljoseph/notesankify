# OneDrive Sync Setup Guide

NotesAnkify now supports automatic synchronization of PDFs from OneDrive. This guide will walk you through setting up OneDrive sync for your NotesAnkify installation.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Azure AD App Registration](#azure-ad-app-registration)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [Usage](#usage)
- [Troubleshooting](#troubleshooting)

## Prerequisites

1. A Microsoft account with OneDrive access
2. PDFs stored in OneDrive that you want to sync
3. NotesAnkify installed on your system

## Azure AD App Registration

To use OneDrive sync, you need to create an Azure AD application. This gives NotesAnkify permission to access your OneDrive files.

### Step 1: Navigate to Azure Portal

1. Go to the [Azure Portal](https://portal.azure.com)
2. Sign in with your Microsoft account
3. Navigate to "Azure Active Directory" (you can search for it in the top search bar)

### Step 2: Register a New Application

1. In the left sidebar, click on **"App registrations"**
2. Click **"New registration"** at the top
3. Fill in the registration form:
   - **Name**: `NotesAnkify` (or any name you prefer)
   - **Supported account types**: Select **"Accounts in any organizational directory and personal Microsoft accounts"**
   - **Redirect URI**: Leave blank (not needed for device code flow)
4. Click **"Register"**

### Step 3: Get Your Client ID

1. After registration, you'll be taken to the app overview page
2. Copy the **"Application (client) ID"** - you'll need this for configuration
3. The **"Directory (tenant) ID"** should be visible too, but for personal accounts, you can use "common"

### Step 4: Configure API Permissions

1. In the left sidebar of your app, click **"API permissions"**
2. Click **"Add a permission"**
3. Select **"Microsoft Graph"**
4. Select **"Delegated permissions"**
5. Search for and add these permissions:
   - `Files.Read`
   - `Files.Read.All`
   - `User.Read`
6. Click **"Add permissions"**
7. (Optional but recommended) Click **"Grant admin consent"** if you have admin rights

### Step 5: Enable Public Client Flow

1. In the left sidebar, click **"Authentication"**
2. Scroll down to **"Advanced settings"**
3. Under **"Allow public client flows"**, toggle **"Yes"**
4. Click **"Save"** at the top

## Configuration

Now that you have your Azure AD app set up, configure NotesAnkify to use it.

### Edit config.yaml

Open your `config.yaml` file and update the OneDrive section:

```yaml
# OneDrive sync configuration
onedrive:
  enabled: true
  # Your Azure AD application client ID from Step 3
  client_id: "YOUR_CLIENT_ID_HERE"
  # Tenant ID (use "common" for personal Microsoft accounts)
  tenant_id: "common"
  # OneDrive folder path to sync (e.g., "Documents/Notes" or "" for root)
  folder_path: "Documents/GoodNotes"
  # Auto-sync interval in minutes (0 = manual sync only)
  sync_interval: 0
```

**Configuration Options:**

- `enabled`: Set to `true` to enable OneDrive sync
- `client_id`: Your Application (client) ID from Azure AD
- `tenant_id`: Use `"common"` for personal Microsoft accounts, or your tenant ID for organizational accounts
- `folder_path`: The path in OneDrive where your PDFs are stored
  - Use `""` for the root OneDrive folder
  - Use `"Documents/Notes"` for a nested folder structure
- `sync_interval`: Auto-sync interval in minutes (currently manual sync only, set to `0`)

## Authentication

Before you can sync files, you need to authenticate NotesAnkify with your Microsoft account.

### First-Time Authentication

Run the following command:

```bash
notesankify --onedrive-auth
```

You'll see output like this:

```
==================================================
OneDrive Authentication Required
==================================================
To sign in, use a web browser to open the page:
  https://microsoft.com/devicelogin

And enter the code:
  ABC123XYZ

==================================================
```

### Steps:

1. Open a web browser
2. Go to the URL shown (usually https://microsoft.com/devicelogin)
3. Enter the code displayed in your terminal
4. Sign in with your Microsoft account
5. Grant permissions to NotesAnkify
6. Return to your terminal - you should see "Authentication successful!"

The authentication token will be saved to `~/.notesankify/onedrive_token.json` and will be reused for future syncs.

### Clear Authentication

If you need to re-authenticate or remove your credentials:

```bash
notesankify --onedrive-clear-auth
```

## Usage

### Sync PDFs from OneDrive

To sync PDFs from OneDrive to your local directory:

```bash
notesankify --sync-onedrive
```

This will:
1. Connect to OneDrive using your saved credentials
2. List all PDF files in your configured folder path (recursively)
3. Download new or updated PDFs to your local PDF directory
4. Skip files that are already up-to-date
5. Show sync statistics when complete

### Sync and Process in One Command

You can combine OneDrive sync with PDF processing:

```bash
notesankify --sync-onedrive --pdf-dir ./pdfs --root-deck "MyNotes"
```

This will:
1. First sync PDFs from OneDrive to `./pdfs`
2. Then process all PDFs and send flashcards to Anki

## Example Workflow

Here's a complete workflow using OneDrive sync:

```bash
# 1. First time: Authenticate with OneDrive
notesankify --onedrive-auth

# 2. Sync PDFs from OneDrive
notesankify --sync-onedrive --pdf-dir ./my-notes

# 3. Process PDFs and send to Anki
notesankify --pdf-dir ./my-notes --root-deck "StudyNotes"

# Or combine steps 2 and 3:
notesankify --sync-onedrive --pdf-dir ./my-notes --root-deck "StudyNotes"
```

## Sync Statistics

When sync completes, you'll see statistics like:

```
Sync completed:
- Files downloaded: 15
- Files skipped: 8
- Bytes downloaded: 25165824 (24.00 MB)
- Duration: 12.5s
```

**What gets skipped?**
- Files that already exist locally with the same size and modification time
- Non-PDF files (only PDFs are synced)

## Troubleshooting

### "OneDrive sync is not enabled"

Make sure you've set `enabled: true` in your `config.yaml` under the `onedrive:` section.

### "client_id is required"

You need to add your Azure AD Application (client) ID to `config.yaml`.

### "Authentication failed"

Try these steps:
1. Clear existing auth: `notesankify --onedrive-clear-auth`
2. Re-authenticate: `notesankify --onedrive-auth`
3. Make sure you're using the correct Microsoft account
4. Verify your Azure AD app has the correct permissions

### "Failed to list OneDrive files"

Possible causes:
- Token has expired - try re-authenticating
- Folder path doesn't exist in OneDrive - check the `folder_path` in config.yaml
- Network connectivity issues

### "Access Denied" errors

Make sure your Azure AD app has the required permissions:
- `Files.Read` or `Files.Read.All`
- `User.Read`

And that you've granted consent for these permissions.

### Files not syncing

Check that:
1. The `folder_path` in config.yaml matches your OneDrive folder structure
2. The files are actually PDFs (only `.pdf` files are synced)
3. You have read access to the files in OneDrive

## Security Notes

- Your authentication token is stored locally in `~/.notesankify/onedrive_token.json`
- This file contains sensitive credentials - keep it secure
- The token has a limited lifetime and will need to be refreshed periodically
- NotesAnkify only requests read permissions to your files
- You can revoke access at any time from your [Microsoft account permissions page](https://account.microsoft.com/privacy/app-access)

## Advanced Configuration

### Custom Sync Directory

You can override the PDF directory during sync:

```bash
notesankify --sync-onedrive --pdf-dir /path/to/custom/directory
```

### Verbose Logging

For detailed sync information:

```bash
notesankify --sync-onedrive --verbose
```

### Debug Mode

For troubleshooting:

```bash
notesankify --sync-onedrive --debug
```

## Next Steps

Once your PDFs are synced, you can process them with NotesAnkify:

- See the [main documentation](README.md) for processing modes and options
- Learn about [flashcard creation](README.md#creating-flashcards)
- Explore [processing modes](README.md#processing-modes)

## Support

If you encounter issues:
- Check the [troubleshooting section](#troubleshooting)
- Review your `notesankify-logs` for detailed error messages
- Report issues on [GitHub](https://github.com/kpauljoseph/notesankify/issues)

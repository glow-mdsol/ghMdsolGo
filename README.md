# ghMDSOLGO

A silly little app to streamline the process of checking a users account for the correct setup and adding if ok.  

It will run the following checks on the user account:
* requires a public email address
* requires 2-FA enabled
* requires Name set on account 

It will use the SSO connection to link a user email to an account, but the user will need to be in the SSO context


## Installation
* Install go
  ```
  brew install go
  ```
* Install the tool
  ```
  go install github.com/glow-mdsol/ghMdsolGo@latest
  ```
* Add your GOBIN path to your path, by adding the following to your `~/.zshrc` or `~/.bashrc`
  ```
  export PATH="~/go/bin:$PATH"
  ```

## Configuration

### Authentication
The app requires a GitHub Token with User and Org permissions. The token is loaded in the following priority order:

1. **Environment Variable**: `GITHUB_AUTH_TOKEN`
2. **Configuration File**: `github_token` field in the config file (see below)
3. **`.netrc` File**: looks for a machine record for `api.github.com` in your [.netrc](https://www.gnu.org/software/inetutils/manual/html_node/The-_002enetrc-file.html) file

### User Configuration File
You can customize settings by creating a configuration file. The tool will automatically look for a config file in the following locations based on your operating system:

* **macOS/Linux**: `~/.config/ghMdsolGo/config.json`
* **Windows**: `%APPDATA%\ghMdsolGo\config.json`

#### Quick Setup with `--init`

The easiest way to create a configuration file is to use the interactive initialization command:

```bash
ghMdsolGo --init
```

This will:
- Prompt you for your default team name
- Optionally prompt for your GitHub personal access token
- Create the config file in the correct location for your OS
- Set appropriate file permissions (Unix/macOS only)

**Example:**
```bash
$ ghMdsolGo --init
Configuration Initialization
============================

Enter default team name [Team Medidata]: Engineering Team

Enter GitHub personal access token (optional):
  Leave empty to use GITHUB_AUTH_TOKEN environment variable or .netrc
Token: ghp_abc123xyz456

✓ Configuration saved to: /Users/username/.config/ghMdsolGo/config.json
✓ File permissions set to 600 (user read/write only)

Configuration summary:
  Default Team: Engineering Team
  GitHub Token: ***configured***
```

#### Rotating/Updating Your GitHub Token

If you need to update or rotate your GitHub token (e.g., for security reasons or token expiration), use the `--rotate-token` command:

```bash
ghMdsolGo --rotate-token
```

This will:
- Preserve your existing configuration settings (like default team)
- Prompt you for a new GitHub token
- Update only the token in the configuration file
- Maintain secure file permissions

**Example:**
```bash
$ ghMdsolGo --rotate-token
Rotate GitHub Token
===================

Current configuration has a token set.

Enter new GitHub personal access token:
  Leave empty to remove the token from config
New Token: ghp_new_token_xyz789

✓ Token updated in: /Users/username/.config/ghMdsolGo/config.json
✓ File permissions verified (600)

✓ GitHub Token: ***configured***
```

**Note:** You can also remove the token from your configuration by leaving the input empty when prompted. This is useful if you want to switch to using an environment variable or .netrc file instead.

#### Manual Configuration

The configuration file should be in JSON format and supports the following options:

```json
{
  "default_team": "Your Team Name",
  "github_token": "ghp_your_github_token_here"
}
```

**Configuration Options:**
- `default_team`: The default team name to use when adding users (defaults to "Team Medidata" if not specified)
- `github_token`: Your GitHub personal access token (optional, only if not using environment variable or .netrc)

**Example Configuration:**
```json
{
  "default_team": "Engineering Team",
  "github_token": "ghp_abc123xyz456"
}
```

**Minimal Configuration (team only):**
```json
{
  "default_team": "Engineering Team"
}
```

**Security Note:** If you store your GitHub token in the config file, make sure the file has appropriate permissions to prevent unauthorized access.

#### Manual File Creation

If you prefer to create the configuration file manually instead of using `--init`:

On macOS/Linux:
```bash
mkdir -p ~/.config/ghMdsolGo
cat > ~/.config/ghMdsolGo/config.json << 'EOF'
{
  "default_team": "Your Team Name",
  "github_token": "ghp_your_token_here"
}
EOF
chmod 600 ~/.config/ghMdsolGo/config.json
```

On Windows (PowerShell):
```powershell
New-Item -ItemType Directory -Force -Path "$env:APPDATA\ghMdsolGo"
Set-Content -Path "$env:APPDATA\ghMdsolGo\config.json" -Value @'
{
  "default_team": "Your Team Name",
  "github_token": "ghp_your_token_here"
}
'@
```


## Usage
Usage of the tool is pretty simple
  ```shell
  Usage is: ghMdsolGo <options> <logins or repository names>
  where options are:
  -a, --add
        Add users to a team (use with --team)
  -A, --add-repo-admin
        Add user as admin collaborator to repository (requires --repo)
  -c, --find-common-teams
        Find teams that have access to ALL specified repositories
  -d, --describe-team
        Show detailed summary of a team (use with --team)
  -h, --help
        Print help
  -L, --list-repo-collaborators
        List collaborators on repository with permissions and added dates (requires --repo)
  -R, --repo string
        Repository name for repo operations
  -r, --reset
        Generate the Reset link
  -s, --team string
        Specified Team (default "Team Medidata")
  
  Note: Without any flags, the tool lists teams for the specified user or repository.
  ```

### Tools

#### User account check
The tool can take a repository name, a user name or a user email (which can only be looked up via the SSO)

In the case of a User we run some tests:
    ```shell
    $ ghMdsolGo someuser
    
    2026/02/02 16:20:21 Using provided login: someuser
    2026/02/02 16:20:21 Processing someuser
    2026/02/02 16:20:21 Validated Pre-requisites for someuser GitHub Email: someuser@email.com
    ```
It will run the following validation checks:
* User has a public email address
* User has a name
* User is a member of the Organisation
* User has 2FA enabled (we don't check for insecure 2fa at the moment)

Once the checks are complete it will list the teams a user has access to
  ```shell
  $ ghMdsolGo someuser
  2022/05/16 11:55:52 Validated Pre-requisites for someuser GitHub Email: someuser@somedomain.com
  2022/05/16 11:55:52 User someuser is a admin of mdsol
  2022/05/16 11:55:53 User someuser is a member of the following teams
  2022/05/16 11:55:53 * Team Alpha (https://github.com/orgs/ORG/teams/team-alpha)
  2022/05/16 11:55:53 * Team Bravo (https://github.com/orgs/ORG/teams/team-bravo)
  ...
  ```
If the argument is a repository, list the teams that have access to a repository (and what level of access they have)
  ```shell
  $ ghMdsolGo somerepo
  2022/05/16 11:55:53 Repository somerepo has the following teams with access:
  2022/05/16 11:55:53 * Team Alpha (https://github.com/orgs/ORG/teams/team-alpha) pull
  2022/05/16 11:55:53 * Team Bravo (https://github.com/orgs/ORG/teams/team-bravo) push
  2022/05/16 11:55:53 * Team Yankee (https://github.com/orgs/ORG/teams/team-yankee) admin
  ...
  ```

#### Reset Invite 
This is a wrapper for removing the SSO connection for a user (for when SSO doesn't link correctly)

This can be done using username; 
  ```shell
  $ ghMdsolGo --reset someuser 
  2022/05/16 12:02:04 Reset Link: https://github.com/orgs/ORG/people/someuser/sso
  ```
Or, via email
  ```shell
  $ ghMdsolGo --reset someuser@somedomain.com 
  2022/05/16 12:02:04 Reset Link: https://github.com/orgs/ORG/people/someuser/sso
  ```

#### Team Matching
Find teams that match a set of requested repos (or teans that are a close match)

##### Case 1: Exact and Close Matches Found
```
Analyzing team access patterns for 5 repositories...
Repositories: [repo1 repo2 repo3 repo4 repo5]

Repository repo1 has 4 teams with access
Repository repo2 has 3 teams with access  
Repository repo3 has 5 teams with access
Repository repo4 has 2 teams with access
Repository repo5 has 4 teams with access

🎯 EXACT MATCHES - Teams with access to ALL 5 repositories:

1. Team: DevOps Team
   Slug: devops-team
   Description: Infrastructure and deployment team
   Access Level: admin
   URL: https://github.com/orgs/mdsol/teams/devops-team
   Coverage: 100% (5/5 repositories)

🔍 CLOSE MATCHES - Teams with access to more than half of the repositories:

1. Team: Security Team
   Slug: security-team
   Description: Application security team
   Access Level: maintain
   URL: https://github.com/orgs/mdsol/teams/security-team
   Coverage: 80.0% (4/5 repositories)
   Missing access to: [repo2]

2. Team: Frontend Team
   Slug: frontend-team
   Description: UI/UX development team
   Access Level: push
   URL: https://github.com/orgs/mdsol/teams/frontend-team
   Coverage: 60.0% (3/5 repositories)
   Missing access to: [repo4, repo5]

📊 SUMMARY: Found 1 exact matches and 2 close matches.
```

##### Case 2: No Exact Matches, Some Close Matches
```
Analyzing team access patterns for 4 repositories...
Repositories: [repo1 repo2 repo3 repo4]

Repository repo1 has 3 teams with access
Repository repo2 has 2 teams with access
Repository repo3 has 4 teams with access
Repository repo4 has 3 teams with access

🎯 EXACT MATCHES: No teams found with access to ALL repositories.

🔍 CLOSE MATCHES - Teams with access to more than half of the repositories:

1. Team: Backend Team
   Slug: backend-team
   Description: Server-side development team
   Access Level: maintain
   URL: https://github.com/orgs/mdsol/teams/backend-team
   Coverage: 75.0% (3/4 repositories)
   Missing access to: [repo2]

📊 SUMMARY: Found 0 exact matches and 1 close matches.
```

##### Case 3: No Matches Found
```
Analyzing team access patterns for 3 repositories...
Repositories: [repo1 repo2 repo3]

Repository repo1 has 2 teams with access
Repository repo2 has 2 teams with access
Repository repo3 has 3 teams with access

🎯 EXACT MATCHES: No teams found with access to ALL repositories.

🔍 CLOSE MATCHES: No teams found with access to more than 1 repositories.

📊 SUMMARY: No teams found with significant access coverage.
To find teams with access to individual repositories, use the --teams flag with each repository name.
```

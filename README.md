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
The app requires a GitHub Token with User and Org permissions; this can be got from:
* a `GITHUB_AUTH_TOKEN` environment variable 
* from a [.netrc](https://www.gnu.org/software/inetutils/manual/html_node/The-_002enetrc-file.html) file.
  * looks for a machine record for `api.github.com`


## Usage
Usage of the tool is pretty simple
  ```shell
  Usage of ghMdsolGo:
  Usage is: ghMdsol <options> <logins or repository names>
  where options are:
  -a, --add
    	Add User to Team ORG
  -c, --find-common-teams
        Find teams that have access to ALL specified repositories
  -h, --help
    	Print help
  -r, --reset
    	Generate the Reset link
  -s, --team string
    	Specified Team (default "Team ORG")
  -t, --teams
    	List User/Repo Teams  
  ```

### Tools

#### User account check
Verify the alignment of the account with expected configuration (public email address, name on account)

    ```shell
    $ ghMdsolGo -check someuser
    
    Your account is non-conformant (no-email), please check the instructions in the room topic.
    2022/05/16 11:00:28 User someuser has no public email
    ```

#### User Team check
List the teams a user has access to
  ```shell
  $ ghMdsolGo -teams someuser
  2022/05/16 11:55:52 Validated Pre-requisites for someuser GitHub Email: someuser@somedomain.com
  2022/05/16 11:55:52 User someuser is a admin of mdsol
  2022/05/16 11:55:53 User someuser is a member of the following teams
  2022/05/16 11:55:53 * Team Alpha (https://github.com/orgs/ORG/teams/team-alpha)
  2022/05/16 11:55:53 * Team Bravo (https://github.com/orgs/ORG/teams/team-bravo)
  ...
  ```
#### Repository Team check
List the teams that have access to a repository (and what level of access they have)
  ```shell
  $ ghMdsolGo -teams somerepo
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
  $ ghMdsolGo -reset someuser 
  2022/05/16 12:02:04 Reset Link: https://github.com/orgs/ORG/people/someuser/sso
  ```
Or, via email
  ```shell
  $ ghMdsolGo -reset someuser@somedomain.com 
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

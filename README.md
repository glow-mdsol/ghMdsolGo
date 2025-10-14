# ghMDSOLGO

A silly little app to streamline the process of checking a users account for the correct setup and adding if ok.  

It will run the following checks on the user account:
* requires a public email address
* requires 2-FA enabled
* requires Name set on account 


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

* User account check
    ```shell
    $ ghMdsolGo -check someuser
    
    Your account is non-conformant (no-email), please check the instructions in the room topic.
    2022/05/16 11:00:28 User someuser has no public email
    ```
* User Team check
  ```shell
  $ ghMdsolGo -teams someuser
  2022/05/16 11:55:52 Validated Pre-requisites for someuser GitHub Email: someuser@somedomain.com
  2022/05/16 11:55:52 User someuser is a admin of mdsol
  2022/05/16 11:55:53 User someuser is a member of the following teams
  2022/05/16 11:55:53 * Team Alpha (https://github.com/orgs/ORG/teams/team-alpha)
  2022/05/16 11:55:53 * Team Bravo (https://github.com/orgs/ORG/teams/team-bravo)
  ...
  ```
* Repository Team check
  ```shell
  $ ghMdsolGo -teams somerepo
  2022/05/16 11:55:53 Repository somerepo has the following teams with access:
  2022/05/16 11:55:53 * Team Alpha (https://github.com/orgs/ORG/teams/team-alpha) pull
  2022/05/16 11:55:53 * Team Bravo (https://github.com/orgs/ORG/teams/team-bravo) push
  2022/05/16 11:55:53 * Team Yankee (https://github.com/orgs/ORG/teams/team-yankee) admin
  ...
  ```
* Reset Invite (for when SSO doesn't link correctly)
  ```shell
  $ ghMdsolGo -reset someuser 
  2022/05/16 12:02:04 Reset Link: https://github.com/orgs/ORG/people/someuser/sso
  ```
* Reset Invite using email to search SSO
  ```shell
  $ ghMdsolGo -reset someuser@somedomain.com 
  2022/05/16 12:02:04 Reset Link: https://github.com/orgs/ORG/people/someuser/sso
  ```
* Find teams that match a set of requested repos
  ```shell
  $ ghMdsolGo -c repo1 repo2 repo3
  Searching for teams that have access to all 3 repositories...
  Repositories: [repo1 repo2 repo3]

  Repository repo1 has 5 teams with access
  Repository repo2 has 3 teams with access  
  Repository repo3 has 4 teams with access

  Found 2 team(s) with access to ALL specified repositories:

  1. Team: DevOps Team
    Slug: devops-team
    Description: Infrastructure and deployment team
    Access Level: admin
    URL: https://github.com/orgs/mdsol/teams/devops-team

  2. Team: Security Team
    Slug: security-team
    Description: Application security team
    Access Level: maintain
    URL: https://github.com/orgs/mdsol/teams/security-team
  These 2 team(s) have access permissions to all 3 repositories listed above.

  $ ghMdsolGo -c repo1 repo2

  Searching for teams that have access to all 2 repositories...
  Repositories: [repo1 repo2]

  Repository repo1 has 3 teams with access
  Repository repo2 has 4 teams with access

  No teams found with access to all specified repositories.
  This means there are no teams that have access to every single repository in the list.
  ```
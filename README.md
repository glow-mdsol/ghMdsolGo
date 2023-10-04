# ghMDSOLGO

A silly little app to streamline the process of checking a users account for the correct setup and adding if ok

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
  -h, --help
    	Print help
  -r, --reset
    	Generate the Reset link
  -s, --team string
    	Specified Team (default "Team ORG")
  -t, --teams
    	List User/Repo Teams  
    ```

### Examples

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

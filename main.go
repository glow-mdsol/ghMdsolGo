package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v43/github"
	"github.com/jdxcode/netrc"
	"golang.org/x/oauth2"
	"rsc.io/getopt"
)

var DOMAINS = []string{"mdsol.com", "shyftanalytics.com", "3ds.com"}

// Default values
const ORG = "mdsol"
const TeamMedidata = "Team Medidata"
const TokenEnvVar = "GITHUB_AUTH_TOKEN"

// Helper function
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// creates the initial contact with GitHub - uses the users netrc to get the
// token
func connect() (context.Context, *http.Client, *github.Client) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal("Unable to get User")
	}
	var token string
	// Priority: 1. Environment variable, 2. Config file, 3. .netrc file

	// 1. Check the environment variable
	token = os.Getenv(TokenEnvVar)

	// 2. Check the config file
	if token == "" {
		token = getGithubToken()
	}

	// 3. Check .netrc file
	if token == "" {
		n, err := netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
		if err != nil {
			log.Fatal("Unable to load token")
		}
		token = n.Machine("api.github.com").Get("password")
	}

	if token == "" {
		log.Fatal("Unable to find a token for access")
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return ctx, tc, client
}

// entityType represents the type of GitHub entity
type entityType int

const (
	entityUnknown entityType = iota
	entityUser
	entityRepository
)

// detectEntityType determines whether an entity slug is a repository or user
// Returns the entity type and resolved login (for users) or repo name (for repos)
func detectEntityType(ctx context.Context, client *github.Client, tc *http.Client, entitySlug string) (entityType, string) {
	// If it contains @, it's definitely a user email, not a repo
	if strings.Contains(entitySlug, "@") {
		login, err := resolveLogin(ctx, tc, &entitySlug)
		if err != nil || login == "" {
			return entityUnknown, ""
		}
		return entityUser, login
	}

	// First check if it's a valid repository in the org
	// Repositories in the org get priority over usernames
	if isRepository(ctx, client, ORG, entitySlug) {
		return entityRepository, entitySlug
	}

	// Then check if it's a valid user
	if isUser(ctx, client, &entitySlug) {
		// Additional check: verify user is a member of the org
		// This prevents treating random GitHub users as valid entities
		_, resp, err := client.Organizations.GetOrgMembership(ctx, entitySlug, ORG)
		if err == nil && resp.StatusCode == 200 {
			return entityUser, entitySlug
		}
		// User exists but is not a member of the org - treat as unknown
		log.Printf("User %s exists but is not a member of organization %s", entitySlug, ORG)
	}

	return entityUnknown, ""
}

func userIsValid(ctx context.Context, client *github.Client, tc *http.Client, userLogin string) (bool, *github.User) {
	ghUser := userPrerequisites(ctx, client, &userLogin)
	// check membership of org
	result, code := meetsOrgPrequisites(ctx, client, ghUser)
	if !result && code == 1 {
		if code == 1 {
			prompt(fmt.Sprintf("User %s is not a member of organisation %s", *ghUser.Login, ORG))
			log.Println("User ", *ghUser.Login, " is not a member of organization ", ORG)
		} else {
			log.Printf("Unable to determine organization membership")
		}
		return false, ghUser
	}
	// check SSO requirements
	result, _ = meetsSSOPrequisites(ctx, tc, ghUser)
	if !result {
		prompt(
			fmt.Sprintf("User %s is not SSO Enabled", *ghUser.Login),
		)
		log.Printf("User %s is not SSO enabled", *ghUser.Login)
		return false, ghUser
	}
	// check 2FA is enabled
	result, code = meets2FAPrerequisites(ctx, client, ghUser)
	if !result {
		prompt(fmt.Sprintf("User %s does not have 2FA enabled", *ghUser.Login))
		log.Printf("User %s does not have 2FA enabled", *ghUser.Login)
		return false, ghUser
	}
	return true, ghUser
}

// Go time!
func main() {
	defaultTeam := getDefaultTeam()
	var teamName = flag.String("team", defaultTeam, "Specified Team")
	var repoName = flag.String("repo", "", "Repository name for repo operations")
	var resetFlag = flag.Bool("reset", false, "Generate the Reset link")
	var findCommonTeams = flag.Bool("find-common-teams", false, "Find teams that have access to ALL specified repositories")
	var addToTM = flag.Bool("add", false, "Add User to Team Medidata")
	var addRepoAdmin = flag.Bool("add-repo-admin", false, "Add user as admin collaborator to repository")
	var listRepoCollaborators = flag.Bool("list-repo-collaborators", false, "List collaborators on repository with permissions and added dates")
	var describeTeam = flag.Bool("describe-team", false, "Show detailed summary of a team")
	var initFlag = flag.Bool("init", false, "Initialize configuration file")
	var rotateTokenFlag = flag.Bool("rotate-token", false, "Rotate/update GitHub token in configuration")
	var help = flag.Bool("help", false, "Print help")
	getopt.Alias("s", "team")
	getopt.Alias("R", "repo")
	getopt.Alias("a", "add")
	getopt.Alias("A", "add-repo-admin")
	getopt.Alias("L", "list-repo-collaborators")
	getopt.Alias("c", "find-common-teams")
	getopt.Alias("r", "reset")
	getopt.Alias("d", "describe-team")
	getopt.Alias("i", "init")
	getopt.Alias("t", "rotate-token")
	getopt.Alias("h", "help")
	getopt.Parse()

	if *initFlag {
		if err := initConfig(); err != nil {
			log.Fatalf("Configuration initialization failed: %v", err)
		}
		os.Exit(0)
	}

	if *rotateTokenFlag {
		if err := rotateToken(); err != nil {
			log.Fatalf("Token rotation failed: %v", err)
		}
		os.Exit(0)
	}

	if *help {
		fmt.Println("ghMdsolGo - GitHub Medidata Organization Management Tool")
		fmt.Println("\nUSAGE:")
		fmt.Println("  ghMdsolGo [options] <usernames/emails or repository names>")
		fmt.Println("\nUSER OPERATIONS:")
		fmt.Println("  -a, --add                    Add users to a team (use with --team)")
		fmt.Println("  -r, --reset                  Generate SSO reset link for users")
		fmt.Println("\nTEAM OPERATIONS:")
		fmt.Println("  -d, --describe-team          Show detailed summary of a team (use with --team)")
		fmt.Println("\nREPOSITORY OPERATIONS:")
		fmt.Println("  -A, --add-repo-admin         Add users as admin collaborators to a repository (requires --repo)")
		fmt.Println("  -L, --list-repo-collaborators")
		fmt.Println("                               List all collaborators on a repository (requires --repo)")
		fmt.Println("  -c, --find-common-teams      Find teams with access to ALL specified repositories")
		fmt.Println("\nOPTIONS:")
		fmt.Printf("  -s, --team <name>            Specify team name (default: '%s')\n", defaultTeam)
		fmt.Println("  -R, --repo <name>            Specify repository name for repo operations")
		fmt.Println("  -i, --init                   Initialize configuration file interactively")
		fmt.Println("  -t, --rotate-token           Rotate/update GitHub token in configuration")
		fmt.Println("  -h, --help                   Show this help message")
		fmt.Println("\nEXAMPLES:")
		fmt.Println("  # Initialize configuration (first time setup)")
		fmt.Println("  ghMdsolGo --init")
		fmt.Println("\n  # Update/rotate GitHub token")
		fmt.Println("  ghMdsolGo --rotate-token")
		fmt.Println("\n  # List teams for a user (default behavior)")
		fmt.Println("  ghMdsolGo user1")
		fmt.Println("\n  # List teams for a repository (default behavior)")
		fmt.Println("  ghMdsolGo my-repo")
		fmt.Println("\n  # Add users to Team Medidata")
		fmt.Println("  ghMdsolGo --add user1 user2@mdsol.com")
		fmt.Println("\n  # Add users to a specific team")
		fmt.Println("  ghMdsolGo --add --team 'Engineering Team' user1 user2")
		fmt.Println("\n  # Generate SSO reset link")
		fmt.Println("  ghMdsolGo --reset username")
		fmt.Println("\n  # Add user as admin to a repository")
		fmt.Println("  ghMdsolGo --add-repo-admin --repo my-repo user1 user2")
		fmt.Println("\n  # List all collaborators on a repository")
		fmt.Println("  ghMdsolGo --list-repo-collaborators --repo my-repo")
		fmt.Println("\n  # Find teams with access to multiple repositories")
		fmt.Println("  ghMdsolGo --find-common-teams repo1 repo2 repo3")
		fmt.Println("\n  # Show detailed summary of a team")
		fmt.Println("  ghMdsolGo --describe-team --team 'Engineering Team'")
		os.Exit(0)
	}
	var userOrRepoList = flag.Args()

	// create a connection
	ctx, tc, client := connect()

	if *describeTeam {
		// Describe a team with detailed summary
		team := getTeamByName(ctx, client, ORG, *teamName)
		log.Printf("Got team '%s' for '%s'", *team.Name, *teamName)
		summary := summarizeTeam(ctx, client, team)
		fmt.Println(summary)
		return
	}

	if *listRepoCollaborators {
		// List collaborators on repository
		if *repoName == "" {
			log.Fatal("--repo flag is required when using --list-repo-collaborators")
		}
		if !isRepository(ctx, client, ORG, *repoName) {
			log.Fatalf("Repository '%s' not found in organization '%s'", *repoName, ORG)
		}

		err := listRepositoryCollaborators(ctx, client, ORG, *repoName)
		if err != nil {
			log.Printf("Error listing collaborators for repository %s: %s", *repoName, err)
		}
		return
	}

	if *addRepoAdmin {
		// Add user as admin collaborator to repository
		if *repoName == "" {
			log.Fatal("--repo flag is required when using --add-repo-admin")
		}
		if !isRepository(ctx, client, ORG, *repoName) {
			log.Fatalf("Repository '%s' not found in organization '%s'", *repoName, ORG)
		}
		if len(userOrRepoList) == 0 {
			log.Fatal("At least one username or email is required")
		}

		for _, entitySlug := range userOrRepoList {
			if entitySlug == "" {
				continue
			}

			// Resolve email to login if needed
			login, err := resolveLogin(ctx, tc, &entitySlug)
			if err != nil {
				log.Printf("Unable to resolve %s: %s", entitySlug, err)
				continue
			}
			if login == "" {
				continue
			}

			// Check if user exists
			if !isUser(ctx, client, &login) {
				log.Printf("User %s not found", login)
				continue
			}

			// Add user as admin collaborator
			err = addUserAsRepoCollaborator(ctx, client, ORG, *repoName, login)
			if err != nil {
				log.Printf("Error adding user %s as admin to repository %s: %s", login, *repoName, err)
			}
		}
		return
	}

	if *findCommonTeams {
		// All arguments should be repository names
		var repoNames []string
		for _, entitySlug := range userOrRepoList {
			if entitySlug == "" {
				continue
			}
			if !isRepository(ctx, client, ORG, entitySlug) {
				log.Printf("Warning: '%s' is not a valid repository in organization '%s', skipping", entitySlug, ORG)
				continue
			}
			repoNames = append(repoNames, entitySlug)
		}

		if len(repoNames) == 0 {
			log.Fatal("No valid repositories found in the provided arguments")
		}

		findAndReportTeamsWithAccessToAllRepos(ctx, client, ORG, repoNames)
		return
	}

	// For all other operations, we need at least one user or repository argument
	if len(userOrRepoList) == 0 {
		log.Fatal("Usage is: ghMdsolGo <options> <logins or repository names>")
	}

	// Process each entity (user or repository)
	for i := 0; i < len(userOrRepoList); i++ {
		entitySlug := userOrRepoList[i]
		if entitySlug == "" {
			// skip empty
			continue
		}

		// Detect what type of entity this is
		entType, resolvedName := detectEntityType(ctx, client, tc, entitySlug)

		switch entType {
		case entityRepository:
			// Handle repository operations
			_, err := checkRepository(ctx, client, ORG, resolvedName)
			if err != nil {
				log.Printf("Can't resolve Repository %s: %s", resolvedName, err)
				continue
			}

			// Default behavior: list teams for repository
			teams, err := getRepositoryTeams(ctx, client, ORG, resolvedName)
			if err != nil {
				log.Printf("Unable to resolve teams for Repository %s: %s", resolvedName, err)
				continue
			}
			log.Printf("Repository %s has the following teams with access:", resolvedName)
			for _, team := range teams {
				log.Printf("* %s (%s) %s", team.name, team.url, team.access)
			}

		case entityUser:
			// Handle user operations
			log.Printf("Processing user %s", resolvedName)

			// Supply the reset URL
			if *resetFlag {
				prompt(fmt.Sprintf("https://github.com/orgs/mdsol/people/%s/sso", resolvedName))
				log.Printf("Reset Link: https://github.com/orgs/mdsol/people/%s/sso", resolvedName)
				continue
			}

			// Check the user is valid
			valid, ghUser := userIsValid(ctx, client, tc, resolvedName)
			if !valid {
				continue
			}

			// Add to team
			if *addToTM {
				team := getTeamByName(ctx, client, ORG, *teamName)
				checkAndAddMember(ctx, client, team, ghUser)
				continue
			}

			// Default behavior (or explicit -t flag): list user's teams
			teams, err := getUserTeams(ctx, tc, ORG, resolvedName)
			if err == nil {
				log.Printf("User %s is a member of the following teams:", resolvedName)
				for _, team := range teams {
					log.Printf("* %s (%s)", team.name, team.url)
				}
			} else {
				log.Println("Unable to get teams: ", err)
			}

		default:
			prompt(fmt.Sprintf("Unable to identify '%s' as a user or repository.", entitySlug))
			log.Printf("Unable to identify '%s' as a user or repository.", entitySlug)
		}
	}
}

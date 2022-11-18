package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/v43/github"
	"github.com/jdxcode/netrc"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
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
	// check the environment variable
	token = os.Getenv(TokenEnvVar)
	if token == "" {
		n, err := netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
		if err != nil {
			log.Fatal("Unable to load token")
		}
		token = n.Machine("github.com").Get("password")
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

// Go time!
func main() {
	var teamName = flag.String("team", TeamMedidata, "Specified Team")
	var resetFlag = flag.Bool("reset", false, "Generate the Reset link")
	var entityTeams = flag.Bool("teams", false, "List User/Repo Teams")
	var addToTM = flag.Bool("add", false, "Add User to Team Medidata")
	var help = flag.Bool("help", false, "Print help")
	getopt.Alias("s", "team")
	getopt.Alias("a", "add")
	getopt.Alias("t", "teams")
	getopt.Alias("r", "reset")
	getopt.Alias("h", "help")
	getopt.Parse()

	if *help == true {
		fmt.Println("Usage is: ghMdsol <options> <logins or repository names>")
		fmt.Println("where options are:")
		getopt.PrintDefaults()
		os.Exit(0)
	}
	var userOrRepoList = flag.Args()
	if len(userOrRepoList) == 0 {
		log.Fatal("Usage is: ghMdsol <options> <logins or repository names>")
	}
	// create a connection
	ctx, tc, client := connect()

	for i := 0; i < len(userOrRepoList); i++ {
		entitySlug := userOrRepoList[i]
		if entitySlug == "" {
			continue
		}
		if isRepository(ctx, client, ORG, entitySlug) {
			// check the repo exists and we have permission
			_, err := checkRepository(ctx, client, ORG, entitySlug)
			if err != nil {
				log.Printf("Can't resolve Repository")
				continue
			}
			// get the teams
			teams, err := getRepositoryTeams(ctx, client, ORG, entitySlug)
			if err != nil {
				log.Printf("Unable to resolve teams for Repostory %s: %s", entitySlug, err)
				continue
			}
			log.Printf("Repository %s has the following teams with access:", entitySlug)
			for _, team := range teams {
				log.Printf("* %s (%s) %s", team.name, team.url, team.access)
			}

		} else if isUser(ctx, client, entitySlug) {
			// Supply the reset URL
			if *resetFlag {
				// copy the reset URL to clipboard
				prompt(fmt.Sprintf("https://github.com/orgs/mdsol/people/%s/sso", entitySlug))
				log.Printf(
					"Reset Link: https://github.com/orgs/mdsol/people/%s/sso",
					entitySlug,
				)
				continue
			}

			// validating prerequisites (exists,
			ghUser := userPrerequisites(ctx, client, &entitySlug)
			result, code := meetsOrgPrequisites(ctx, client, ghUser)
			if !result && code == 1 {
				if code == 1 {
					prompt(fmt.Sprintf("User %s is not a member of organisation %s", *ghUser.Login, ORG))
					log.Println("User ", *ghUser.Login, " is not a member of organization ", ORG)
				} else {
					log.Printf("Unable to determine organization membership")
				}
				continue
			}
			result, code = meetsSSOPrequisites(ctx, tc, ghUser)
			if !result {
				prompt(
					fmt.Sprintf("User %s is not SSO Enabled", *ghUser.Login),
				)
				log.Printf("User %s is not SSO enabled", *ghUser.Login)
				continue
			}

			if *addToTM {
				team := getTeamByName(ctx, client, ORG, *teamName)
				checkAndAddMember(ctx, client, team, ghUser)
				continue
			}

			// check membership of team
			if *entityTeams {
				teams, err := getUserTeams(ctx, tc, ORG, entitySlug)
				if err == nil {
					log.Printf("User %s is a member of the following teams", entitySlug)
					for _, team := range teams {
						log.Printf("* %s (%s)", team.name, team.url)
					}
				} else {
					log.Println("Unable to get teams: ", err)
				}
				continue
			}
		} else {
			prompt(fmt.Sprintf("Account or Repository %s not found.", entitySlug))
		}
	}
}

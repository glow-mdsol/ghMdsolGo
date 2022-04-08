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
)

var DOMAINS = []string{"mdsol.com", "shyftanalytics.com", "3ds.com"}

// Default values
const ORG = "mdsol"
const TeamMedidata = "Team Medidata"

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
	n, err := netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
	if err != nil {
		log.Fatal("Unable to load token")
	}
	token := n.Machine("github.com").Get("password")

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
	var checkFlag = flag.Bool("check", false, "Check the account(s)")
	var userTeams = flag.Bool("teams", false, "List User Teams")
	var repoTeams = flag.Bool("repo-teams", false, "List Teams For Repo	")
	//var repoName = flag.String("repository", "", "Name of the new repository")
	//var repoDescription = flag.String("description", "", "Description for the new repository")
	//var templateRepo = flag.String("template", "", "Template repository to use")
	flag.Parse()

	var userOrRepoList = flag.Args()
	if len(userOrRepoList) == 0 {
		fmt.Println("Usage is: ghMdsol <options> <logins or repository names>")
		fmt.Println("where options are:")
		flag.PrintDefaults()
		os.Exit(0)
	}
	// create a connection
	ctx, tc, client := connect()

	for i := 0; i < len(userOrRepoList); i++ {
		if *repoTeams {
			// treat arguments as repos
			var repoId = userOrRepoList[i]
			// check the repo exists and we have permission
			_, err := checkRepository(ctx, client, ORG, repoId)
			if err != nil {
				log.Printf("Can't resolve Repository")
				continue
			}
			// get the teams
			teams, err := getRepositoryTeams(ctx, client, ORG, repoId)
			if err != nil {
				log.Printf("Unable to resolve teams for Repostory %s: %s", repoId, err)
				continue
			}
			log.Printf("Repository %s has the following teams with access:", repoId)
			for _, team := range teams {
				log.Printf("* %s (%s) %s", team.name, team.url, team.access)
			}

		} else {
			var userId = userOrRepoList[i]
			if userId != "" {
				// Supply the reset URL
				if *resetFlag {
					log.Printf(
						"Reset Link: https://github.com/orgs/mdsol/people/%s/sso",
						userId,
					)
					continue
				}

				// validating prerequisites (exists,
				ghUser := userPrerequisites(ctx, client, &userId)
				orgPrequisites(ctx, client, ghUser)
				ssoPrequisites(ctx, tc, ghUser)
				if *checkFlag {
					// just check the profile
					continue
				}

				if *userTeams {
					teams, err := getUserTeams(ctx, tc, ORG, userId)
					if err == nil {
						log.Printf("User %s is a member of the following teams", userId)
						for _, team := range teams {
							log.Printf("* %s (%s)", team.name, team.slug)
						}
					} else {
						log.Println("Unable to get teams: ", err)
					}
					continue
				}
				// check membership of team
				//var org *github.Organization
				//org, resp, err = client.Organizations.Get(ctx, ORG)
				team := getTeamByName(ctx, client, ORG, *teamName)

				checkAndAddMember(ctx, client, team, ghUser)
			}
		}
	}
	//} else {
	//	teams := []string{"Team Medidata"}
	//	if *teamName != ""{
	//		teams = append(teams, *teamName)
	//	}
	//	// repo mods
	//	repoInfo := repositoryInfo{
	//		owner:        ORG,
	//		name:         *repoName,
	//		description:  *repoDescription,
	//		teams:        teams,
	//		templateRepo: "",
	//	}
	//	if *templateRepo != "" {
	//		repoInfo.templateRepo = *templateRepo
	//	}
	//	created, err := createRepository(ctx, client, repoInfo)
	//	if err != nil {
	//		log.Fatal("Unable to create repository: ", err)
	//	} else {
	//		log.Println("Created ", created.Name)
	//	}
	//	enabled, err := enableVulnerabilityAlerts(ctx, client, repoInfo.owner, repoInfo.name)
	//	if enabled == false {
	//		log.Fatal("Unable to enable vulnerability alerts:", err )
	//	}
	//}
}

package main

import (
	"context"
	"flag"
	"github.com/google/go-github/v42/github"
	"log"
	"net/http"
	"os/user"
	"path/filepath"

	"github.com/jdxcode/netrc"
	"golang.org/x/oauth2"
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
	//var repoName = flag.String("repository", "", "Name of the new repository")
	//var repoDescription = flag.String("description", "", "Description for the new repository")
	//var templateRepo = flag.String("template", "", "Template repository to use")
	flag.Parse()

	var userList = flag.Args()
	if len(userList) == 0 {
		log.Fatal("Need one or more User IDs")
	}
	// create a connection
	ctx, tc, client := connect()

	for i := 0; i < len(userList); i++ {
		var userId = userList[i]
		if userId != "" {
			// Supply the reset URL
			if *resetFlag {
				log.Printf("Reset Link: https://github.com/orgs/mdsol/people/%s/sso", userId)
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

			// check membership of team
			//var org *github.Organization
			//org, resp, err = client.Organizations.Get(ctx, ORG)
			team := getTeamByName(ctx, client, ORG, *teamName)

			checkAndAddMember(ctx, client, team, ghUser)
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

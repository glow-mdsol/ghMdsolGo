package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/jdxcode/netrc"
	"golang.org/x/oauth2"
	"log"
	"os/user"
	"path/filepath"
	"strings"
)

var DOMAINS = []string{"mdsol.com", "shyftanalytics.com", "3ds.com"}

// Default values
const ORG = "mdsol"
const TeamMedidata = "Team Medidata"

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func main() {
	userId := flag.String("username", "", "User ID")
	teamName := flag.String("team", TeamMedidata, "Specified Team")
	flag.Parse()
	if *userId == "" {
		log.Fatal("Need a User ID")
	}

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

	// list all repositories for the authenticated user
	ghUser, resp, err := client.Users.Get(ctx, *userId)
	if err != nil {
		log.Fatal(fmt.Printf("Error while getting user: %s", err))
	}
	if resp.StatusCode == 404 {
		log.Fatal(fmt.Printf("User %s not found", userId))
	}
	if ghUser.Email == nil {
		log.Fatal("User ", *userId, " has no public email")
	}
	parts := strings.Split(*ghUser.Email, "@")
	conformant := contains(DOMAINS, parts[1])
	if !conformant {
		log.Fatal("User", userId, "has non-conformant email address", ghUser.Email)
	}
	log.Println("Validated Pre-requisites for", *userId, "GitHub Email:", *ghUser.Email)
	// check to see if the user is in the org
	var orgMembership *github.Membership
	orgMembership, resp, err = client.Organizations.GetOrgMembership(ctx, *ghUser.Login, ORG)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			log.Fatal("User ", *ghUser.Login, " is not a member of organization ", ORG)
		} else {
			log.Fatal("Membership lookup failed for ", *ghUser.Email, " error: ", err)
		}
	}
	log.Println("User", *ghUser.Login, "is a", *orgMembership.Role, "of", ORG)
	// check whether SSO enabled
	enabled, err := userIsSSO(ctx, tc, ORG, *ghUser.Login)
	if err != nil || !enabled {
		log.Fatal("User ", *ghUser.Login, " is not SSO enabled")
	}
	// check membership of team medidata
	//var org *github.Organization
	//org, resp, err = client.Organizations.Get(ctx, ORG)
	opt := &github.ListOptions{
		PerPage: 50,
	}
	var team *github.Team

	for {
		var teams []*github.Team
		teams, resp, err = client.Teams.ListTeams(ctx, ORG, opt)
		if err != nil {
			log.Fatal("Error retrieving Team")
		}
		for _, teamIter := range teams {
			if *teamIter.Name == *teamName {
				team = teamIter
				break
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	if team == nil {
		log.Fatal("Unable to find team", teamName)
	}

	var teamMembership *github.Membership
	teamMembership, resp, err = client.Teams.GetTeamMembership(ctx, *team.ID, *ghUser.Login)
	if teamMembership == nil {
		opts := github.TeamAddTeamMembershipOptions{Role: "member"}
		_, resp, err = client.Teams.AddTeamMembership(ctx, *team.ID, *ghUser.Login, &opts)
		if err != nil {
			log.Fatal("Error adding user", *ghUser.Login, " to Team", *team.Name, ": ", err)
		}
		log.Println("User", *ghUser.Login, "added to", *team.Name)
	} else {
		log.Println("User", *ghUser.Login, "is already a member of", *team.Name)
	}
}

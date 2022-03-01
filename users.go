package main

import (
	"fmt"
	github "github.com/google/go-github/v42/github"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"strings"
)

// generate slugs for github enitities (teams esp.)
func slugify(teamName string) (slugged string) {
	slugged = strings.ReplaceAll(strings.ToLower(teamName), " ", "-")
	return
}

// check the prerequisites for a users
func userPrerequisites(ctx context.Context, client *github.Client, userId *string) *github.User {
	// list all repositories for the authenticated user
	ghUser, resp, err := client.Users.Get(ctx, *userId)
	if err != nil {
		log.Fatal(fmt.Printf("Error while getting user: %s", err))
	}
	if resp.StatusCode == 404 {
		log.Fatal(fmt.Printf("User %s not found", *userId))
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
	return ghUser
}

// check the users organisational requirements
func orgPrequisites(ctx context.Context, client *github.Client, ghUser *github.User) {
	// check to see if the user is in the org
	var orgMembership *github.Membership
	orgMembership, resp, err := client.Organizations.GetOrgMembership(ctx, *ghUser.Login, ORG)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			log.Fatal("User ", *ghUser.Login, " is not a member of organization ", ORG)
		} else {
			log.Fatal("Membership lookup failed for ", *ghUser.Email, " error: ", err)
		}
	}
	log.Println("User", *ghUser.Login, "is a", *orgMembership.Role, "of", ORG)
}

// check whether the user is SSO enabled
func ssoPrequisites(ctx context.Context, tc *http.Client, ghUser *github.User) {
	enabled, err := userIsSSO(ctx, tc, ORG, *ghUser.Login)
	if err != nil || !enabled {
		log.Fatal("User ", *ghUser.Login, " is not SSO enabled")
	}
}

// get a team by name (using the generated slug)
func getTeamByName(ctx context.Context, client *github.Client, org, teamName string) *github.Team {
	team, _, err := client.Teams.GetTeamBySlug(ctx, org, slugify(teamName))
	if err != nil {
		log.Fatal("Unable to find team ", teamName, " - ", err)
	}
	return team
}

// check the prerequisites and if satisfied add the user to the team
func checkAndAddMember(ctx context.Context, client *github.Client, team *github.Team, ghUser *github.User) {
	var teamMembership *github.Membership
	teamMembership, response, err := client.Teams.GetTeamMembershipByID(ctx,
		*team.Organization.ID,
		*team.ID,
		*ghUser.Login)
	// check for 404
	if err != nil && response.StatusCode != 404 {
		log.Fatal("Unable to check team membership: ", err)
	}
	if teamMembership == nil {
		opts := github.TeamAddTeamMembershipOptions{Role: "member"}
		_, _, err = client.Teams.AddTeamMembershipByID(ctx,
			*team.Organization.ID,
			*team.ID,
			*ghUser.Login,
			&opts)
		if err != nil {
			log.Fatal("Error adding user", *ghUser.Login, " to Team", *team.Name, ": ", err)
		}
		log.Println("User", *ghUser.Login, "added to", *team.Name)
	} else {
		log.Println("User", *ghUser.Login, "is already a member of", *team.Name)
	}
}

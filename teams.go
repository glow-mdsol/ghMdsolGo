package main

import (
	"fmt"
	"github.com/google/go-github/v43/github"
	"golang.org/x/net/context"
	"log"
)

func isTeam(ctx context.Context, client *github.Client, org, entityId string) bool {
	_, resp, err := client.Organizations.Get(ctx, org)
	if err != nil || resp.StatusCode == 404 {
		return false
	}
	_, resp, err = client.Teams.GetTeamBySlug(ctx, org, entityId)
	if err != nil {
		log.Fatal("Unable to find team ", entityId, " - ", err)
	}
	return resp.StatusCode == 200
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
		prompt(fmt.Sprintf("User %s added to %s", *ghUser.Login, *team.Name))
		log.Println("User", *ghUser.Login, "added to", *team.Name)
	} else {
		log.Println("User", *ghUser.Login, "is already a member of", *team.Name)
	}
}

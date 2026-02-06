package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v43/github"
	"golang.org/x/net/context"
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

// summarizeTeam provides a summary of team information including member count and repository access
func summarizeTeam(ctx context.Context, client *github.Client, team *github.Team) string {
	var summary strings.Builder

	// Get team members count
	membersOpts := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	memberCount := 0
	for {
		members, resp, err := client.Teams.ListTeamMembersByID(ctx, *team.Organization.ID, *team.ID, membersOpts)
		if err != nil {
			log.Printf("Error getting team members: %v", err)
			break
		}
		memberCount += len(members)
		if resp.NextPage == 0 {
			break
		}
		membersOpts.Page = resp.NextPage
	}

	// Get team repositories
	reposOpts := &github.ListOptions{PerPage: 100}
	allRepos := make(map[string][]*github.Repository) // grouped by permission
	totalRepoCount := 0
	for {
		repos, resp, err := client.Teams.ListTeamReposByID(ctx, *team.Organization.ID, *team.ID, reposOpts)
		if err != nil {
			log.Printf("Error getting team repositories: %v", err)
			break
		}
		for _, repo := range repos {
			permission := "read" // default
			if repo.Permissions != nil {
				if repo.Permissions["admin"] {
					permission = "admin"
				} else if repo.Permissions["maintain"] {
					permission = "maintain"
				} else if repo.Permissions["push"] {
					permission = "write"
				} else if repo.Permissions["triage"] {
					permission = "triage"
				}
			}
			allRepos[permission] = append(allRepos[permission], repo)
			totalRepoCount++
		}
		if resp.NextPage == 0 {
			break
		}
		reposOpts.Page = resp.NextPage
	}

	// Build summary
	summary.WriteString(fmt.Sprintf("Team: %s\n", *team.Name))
	if team.Description != nil && *team.Description != "" {
		summary.WriteString(fmt.Sprintf("Description: %s\n", *team.Description))
	}
	summary.WriteString(fmt.Sprintf("Members: %d\n", memberCount))
	summary.WriteString(fmt.Sprintf("Total Repositories: %d\n", totalRepoCount))

	if totalRepoCount > 0 {
		summary.WriteString("\nRepositories by Permission:\n")

		// Order of permissions to display
		permissionOrder := []string{"admin", "maintain", "write", "triage", "read"}

		for _, perm := range permissionOrder {
			repos, exists := allRepos[perm]
			if !exists || len(repos) == 0 {
				continue
			}

			summary.WriteString(fmt.Sprintf("\n  %s (%d):\n", perm, len(repos)))

			limit := 10
			for i, repo := range repos {
				if i >= limit {
					summary.WriteString("    ...\n")
					break
				}
				summary.WriteString(fmt.Sprintf("    - %s\n", *repo.Name))
			}
		}
	}

	return summary.String()
}

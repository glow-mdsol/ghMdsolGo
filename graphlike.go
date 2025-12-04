package main

import (
	"context"
	"log"
	"net/http"

	"github.com/shurcooL/githubv4"
)

type teamInfo struct {
	name        string
	orgId       string
	teamId      string
	description string
	slug        string
	url         string
	access      string
}

type samlNode struct {
	User struct {
		Login string
	}
	Guid         string `graphql:"guid"`
	SamlIdentity struct {
		NameId string
	}
}

type teamNode struct {
	ID          string
	Name        string
	Description string
	Slug        string
	URL         string
}

// Checks if the user is SSO enabled
func userIsSSO(ctx context.Context, httpClient *http.Client, org string, login string) (bool, error) {
	var q struct {
		Organization struct {
			SamlIdentityProvider struct {
				ExternalIdentities struct {
					Nodes    []samlNode
					PageInfo struct {
						EndCursor   githubv4.String
						HasNextPage githubv4.Boolean
					}
				} `graphql:"externalIdentities(first: 100, after: $cursor)"`
			}
		} `graphql:"organization(login: $login)"`
	}
	variables := map[string]interface{}{
		"login":  githubv4.String(org),
		"cursor": (*githubv4.String)(nil), // Null after argument to get first page.
	}
	client := githubv4.NewClient(httpClient)
	for {
		err := client.Query(ctx, &q, variables)

		if err != nil {
			log.Println("Got error querying SSO:", err)
			return false, err
		}
		for _, node := range q.Organization.SamlIdentityProvider.ExternalIdentities.Nodes {
			if node.User.Login == login {
				// found the user
				return true, nil
			}
		}
		if !q.Organization.SamlIdentityProvider.ExternalIdentities.PageInfo.HasNextPage {
			break
		}
		variables["cursor"] = githubv4.NewString(q.Organization.SamlIdentityProvider.ExternalIdentities.PageInfo.EndCursor)
	}
	return false, nil
}

func findUserByEmail(ctx context.Context, httpClient *http.Client, org string, email string) (string, error) {
	var q struct {
		Organization struct {
			SamlIdentityProvider struct {
				ExternalIdentities struct {
					TotalCount githubv4.Int
					Nodes      []samlNode
				} `graphql:"externalIdentities(first: 100, userName: $email, after: $cursor)"`
			}
		} `graphql:"organization(login: $login)"`
	}
	variables := map[string]interface{}{
		"login":  githubv4.String(org),
		"email":  githubv4.String(email),
		"cursor": (*githubv4.String)(nil), // Null after argument to get first page.
	}
	client := githubv4.NewClient(httpClient)
	for {
		err := client.Query(ctx, &q, variables)

		if err != nil {
			log.Println("Got error querying email:", err)
			return "", err
		}
		if q.Organization.SamlIdentityProvider.ExternalIdentities.TotalCount != 0 {
			return q.Organization.SamlIdentityProvider.ExternalIdentities.Nodes[0].User.Login, nil
		} else {
			return "", nil
		}
	}
}

// Get the Team Details
//func getOrganisationAndTeamId(ctx context.Context, httpClient *http.Client,
//	org string, teamName string) (*teamInfo, error) {
//	var q struct {
//		Organization struct {
//			ID    string
//			Login string
//			Teams struct {
//				totalCount int64
//				nodes      []teamNode
//			} `graphql:"teams(query: $teamName, first: 1)"`
//		} `graphql:"organization(login: $login)"`
//	}
//	variables := map[string]interface{}{
//		"login":    githubv4.String(org),
//		"teamName": githubv4.String(teamName),
//	}
//	client := githubv4.NewClient(httpClient)
//	err := client.Query(ctx, &q, variables)
//
//	if err != nil {
//		log.Println("Got error querying Org/Team ID:", err)
//		return nil, err
//	}
//	if q.Organization.Teams.totalCount != 1 {
//		if q.Organization.Teams.totalCount == 0 {
//			log.Println("Team", teamName, " not found:")
//		} else {
//			log.Println("Got ", q.Organization.Teams.totalCount, " matches for Team ", teamName)
//		}
//		return nil, err
//	}
//	team := teamInfo{name: teamName,
//		orgId:  q.Organization.ID,
//		teamId: q.Organization.Teams.nodes[0].ID}
//	return &team, nil
//}

// Get a Users Teams
func getUserTeams(ctx context.Context, httpClient *http.Client,
	org string, userLogin string) ([]teamInfo, error) {
	log.Printf("getUserTeams called with org=%s, userLogin=%s", org, userLogin)
	var q struct {
		Organization struct {
			ID    string
			Login string
			Teams struct {
				TotalCount int64
				Nodes      []teamNode
			} `graphql:"teams(first: $first, userLogins: $userLogin)"`
		} `graphql:"organization(login: $org)"`
	}
	variables := map[string]interface{}{
		"org":       githubv4.String(org),
		"userLogin": []githubv4.String{githubv4.String(userLogin)},
		"first":     githubv4.Int(100),
	}
	client := githubv4.NewClient(httpClient)
	err := client.Query(ctx, &q, variables)
	if err != nil {
		log.Println("Got error querying Team Lists:", err)
		return nil, err
	}
	log.Printf("getUserTeams found %d teams for user %s", q.Organization.Teams.TotalCount, userLogin)
	var teams []teamInfo
	for _, team := range q.Organization.Teams.Nodes {
		teams = append(teams, teamInfo{
			name:        team.Name,
			orgId:       q.Organization.ID,
			teamId:      team.ID,
			slug:        team.Slug,
			description: team.Description,
			url:         team.URL,
		})
	}
	return teams, nil
}

package main

import (
	"context"
	"github.com/shurcooL/githubv4"
	"log"
	"net/http"
)

type teamInfo struct {
	name   string
	orgId  string
	teamId string
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
	id   string
	name string
}

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

func getOrganisationAndTeamId(ctx context.Context, httpClient *http.Client,
	org string, teamName string) (*teamInfo, error) {
	var q struct {
		Organization struct {
			ID    string
			Login string
			Teams struct {
				totalCount int64
				nodes      []teamNode
			} `graphql:"teams(query: $teamName, first: 1)"`
		} `graphql:"organization(login: $login)"`
	}
	variables := map[string]interface{}{
		"login":    githubv4.String(org),
		"teamName": githubv4.String(teamName),
	}
	client := githubv4.NewClient(httpClient)
	err := client.Query(ctx, &q, variables)

	if err != nil {
		log.Println("Got error querying Org/Team ID:", err)
		return nil, err
	}
	if q.Organization.Teams.totalCount != 1 {
		if q.Organization.Teams.totalCount == 0 {
			log.Println("Team", teamName, " not found:")
		} else {
			log.Println("Got ", q.Organization.Teams.totalCount, " matches for Team ", teamName)
		}
		return nil, err
	}
	team := teamInfo{name: teamName,
		orgId:  q.Organization.ID,
		teamId: q.Organization.Teams.nodes[0].id}
	return &team, nil
}

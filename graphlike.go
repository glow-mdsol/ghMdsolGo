package main

import (
	"context"
	"github.com/shurcooL/githubv4"
	"log"
	"net/http"
)

type samlNode struct {
	User struct {
		Login string
	}
	Guid string `graphql:"guid"`
	SamlIdentity struct {
		NameId string
	}
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

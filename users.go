package main

import (
	"fmt"
	. "github.com/google/go-github/v43/github"
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

func isUser(ctx context.Context, client *Client, entitySlug string) bool {
	_, resp, _ := client.Users.Get(ctx, entitySlug)
	if resp.StatusCode == 200 {
		return true
	}
	return false
}

// check the prerequisites for a users
func userPrerequisites(ctx context.Context, client *Client, userId *string) *User {
	// list all repositories for the authenticated user
	ghUser, resp, err := client.Users.Get(ctx, *userId)
	if err != nil {
		log.Fatal(fmt.Printf("Error while getting user: %s", err))
	}
	if resp.StatusCode == 404 {
		log.Fatal(fmt.Printf("User %s not found", *userId))
	}
	if ghUser.Email == nil {
		prompt(fmt.Sprintf("The account %s is non-conformant (no-public-email), please "+
			"check the instructions in the room topic. ( fix on https://github.com/settings/profile )", *userId))
		log.Fatal("User ", *userId, " has no public email")
	}
	parts := strings.Split(*ghUser.Email, "@")
	conformant := contains(DOMAINS, parts[1])
	if !conformant {
		prompt(fmt.Sprintf("The account %s (email %s) is non-conformant (incorrect mail domain), "+
			"please check the instructions in the room topic.", *userId, *ghUser.Email))
		log.Fatal("User", userId, "has non-conformant email address", ghUser.Email)
	}
	log.Println("Validated Pre-requisites for", *userId, "GitHub Email:", *ghUser.Email)
	return ghUser
}

// check the users organisational requirements
func meetsOrgPrequisites(ctx context.Context, client *Client, ghUser *User) (bool, int) {
	// check to see if the user is in the org
	var orgMembership *Membership
	orgMembership, resp, err := client.Organizations.GetOrgMembership(ctx, *ghUser.Login, ORG)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, 1
		} else {
			return false, 2
		}
	}
	log.Println("User", *ghUser.Login, "is a", *orgMembership.Role, "of", ORG)
	return true, 0
}

// check whether the user is SSO enabled
func meetsSSOPrequisites(ctx context.Context, tc *http.Client, ghUser *User) (bool, int) {
	enabled, err := userIsSSO(ctx, tc, ORG, *ghUser.Login)
	if err != nil || !enabled {
		return false, 1
	}
	return true, 0
}

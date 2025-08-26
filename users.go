package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v43/github"
	"golang.org/x/net/context"
)

// slugify - generate slugs for github enitities (teams esp.)
func slugify(teamName string) (slugged string) {
	slugged = strings.ReplaceAll(strings.ToLower(teamName), " ", "-")
	return
}

// isUser - confirm that the entitySlug refers to a user
func isUser(ctx context.Context, client *github.Client, entitySlug *string) bool {
	_, resp, _ := client.Users.Get(ctx, *entitySlug)
	return resp.StatusCode == 200
}

// resolveLogin - resolve an email or login to a user
func resolveLogin(ctx context.Context, tc *http.Client, entitySlug *string) (string, error) {
	// try to resolve user by email (only in context of Org)
	if strings.Contains(*entitySlug, "@") {
		// assume email
		login, err := findUserByEmail(ctx, tc, ORG, *entitySlug)
		if err != nil {
			log.Printf("Unable to resolve email %s: %s", login, err)
			return "", err
		}
		if login == "" {
			log.Printf("Unable to resolve email %s to valid user", *entitySlug)
			return "", nil
		}
		log.Printf("Resolved email %s to user %s", *entitySlug, login)
		return login, nil
	} else {
		return *entitySlug, nil
	}
}

// userPrerequisites - check the prerequisites for a users
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
		prompt(fmt.Sprintf("The account %s is non-conformant (no-public-email), please "+
			"check the instructions in the room topic. ( fix on https://github.com/settings/profile )", *userId))
		log.Fatal("User ", *userId, " has no public email")
	}
	if ghUser.Name == nil {
		prompt(fmt.Sprintf("The account %s is non-conformant (no-name), please "+
			"check the instructions in the room topic. ( fix on https://github.com/settings/profile )", *userId))
		log.Fatal("User ", *userId, " has no public name")
	}

	parts := strings.Split(*ghUser.Email, "@")
	conformant := contains(DOMAINS, parts[1])
	if !conformant {
		prompt(fmt.Sprintf("The account %s (email %s) is non-conformant (incorrect mail domain), "+
			"please check the instructions in the room topic.", *userId, *ghUser.Email))
		log.Fatal("User", userId, "has non-conformant email address", ghUser.Email)
	}
	// This doesn't work unless the user is a member of the org
	// if ghUser.TwoFactorAuthentication == nil || !*ghUser.TwoFactorAuthentication {
	// 	prompt(fmt.Sprintf("The account %s is non-conformant (no-2FA), please "+
	// 		"check the instructions in the room topic. ( fix on https://github.com/settings/security )", *userId))
	// 	log.Fatal("User ", *userId, "has no 2FA enabled")
	// }

	log.Println("Validated Pre-requisites for", *userId, "GitHub Email:", *ghUser.Email)
	return ghUser
}

// meetsOrgPrequisites - check the users organisational requirements
func meetsOrgPrequisites(ctx context.Context, client *github.Client, ghUser *github.User) (bool, int) {
	// check to see if the user is in the org
	var orgMembership *github.Membership
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

// meetsSSOPrequisites - check whether the user is SSO enabled
func meetsSSOPrequisites(ctx context.Context, tc *http.Client, ghUser *github.User) (bool, int) {
	enabled, err := userIsSSO(ctx, tc, ORG, *ghUser.Login)
	if err != nil || !enabled {
		return false, 1
	}
	return true, 0
}

// meets2FAPrerequisites - check whether the user has 2FA enabled
// func meets2FAPrerequisites(ctx context.Context, client *Client, ghUser *User) (bool, int) {
// 	membership, resp, err := client.Organizations.GetOrgMembership(ctx, *ghUser.Login, ORG)
// 	if err != nil {
// 		if resp != nil && resp.StatusCode == 404 {
// 			return false, 1
// 		} else {
// 			log.Printf("Error checking 2FA for user %s: %s", *ghUser.Login, err)
// 			return false, 2
// 		}
// 	}
// 	if *membership.Role != "member" {
// 		return false, 3
// 	}

// 	return true, 0
// }
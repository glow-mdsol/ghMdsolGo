package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v43/github"
	"golang.org/x/net/context"
)

type repositoryInfo struct {
	owner        string
	name         string
	description  string
	teams        []string
	templateRepo string
}

// checkRepository - check if a repository exists
func checkRepository(ctx context.Context,
	client *github.Client, owner, repositoryName string) (*repositoryInfo, error) {
	repository, _, err := client.Repositories.Get(ctx, owner, repositoryName)
	if err != nil {
		log.Println("Unable to detect whether repository exists:", err)
		return nil, err
	}
	info := repositoryInfo{
		owner:        *repository.Owner.Login,
		name:         *repository.Name,
		teams:        nil,
		templateRepo: "",
	}
	if repository.Description != nil {
		info.description = *repository.Description
	}
	return &info, nil
}

// isRepository - confirms that the input is the name of a repository within the org
func isRepository(ctx context.Context, client *github.Client, org, entitySlug string) bool {
	// repo will never have a @ in it
	if strings.Contains(entitySlug, "@") {
		return false
	}
	_, resp, err := client.Organizations.Get(ctx, org)
	if err != nil || resp.StatusCode == 404 {
		return false
	}
	_, resp, err = client.Repositories.Get(ctx, org, entitySlug)
	if err == nil && resp.StatusCode == 200 {
		return true
	}
	return false
}

// createRepository - create a new repository within the org
func createRepository(ctx context.Context,
	client *github.Client,
	info repositoryInfo) (*github.Repository, error) {
	exists, _, err := client.Repositories.Get(ctx, info.owner, info.name)
	if err != nil {
		log.Println("Unable to detect whether repository exists:", err)
		return nil, err
	}
	if exists != nil {
		log.Println("Repository exists")
		return nil, nil
	}

	repository := &github.Repository{
		Name:        github.String(info.name),
		Private:     github.Bool(true),
		Description: github.String(info.description)}

	if info.templateRepo != "" {
		template, _, err := client.Repositories.Get(ctx, info.owner, info.templateRepo)
		if err != nil {
			log.Println("Unable to locate template dir ", info.templateRepo)
		} else {
			repository.TemplateRepository = template
		}
	}

	repo, _, err := client.Repositories.Create(ctx, info.owner, repository)
	if err != nil {
		log.Fatal("Creating repo failed:", err)
	}
	return repo, nil
}

func enableVulnerabilityAlerts(ctx context.Context, client *github.Client, owner, repository string) (bool, error) {
	enabled, _, err := client.Repositories.GetVulnerabilityAlerts(ctx, owner, repository)
	if err != nil {
		log.Println("Unable to find repository", err)
		return false, err
	}
	if enabled {
		log.Println("Repository ", repository, "already enabled")
		return false, nil
	}
	_, err = client.Repositories.EnableVulnerabilityAlerts(ctx, owner, repository)
	if err != nil {
		log.Println("Unable to enable vulnerability alerts for repository", err)
		return false, err
	}
	return true, nil
}

// getRepositoryTeams - get the teams associated with a repository
func getRepositoryTeams(ctx context.Context, client *github.Client, owner, repositoryName string) ([]teamInfo, error) {

	// Check the repo exists
	_, _, err := client.Repositories.Get(ctx, owner, repositoryName)
	if err != nil {
		return nil, err
	}

	var listOptions = github.ListOptions{PerPage: 100}
	repoTeams, _, err := client.Repositories.ListTeams(ctx, owner, repositoryName, &listOptions)
	if err != nil {
		return nil, err
	}
	var teams []teamInfo
	for _, team := range repoTeams {
		teams = append(teams, teamInfo{
			name:        *team.Name,
			description: *team.Description,
			slug:        *team.Slug,
			url:         *team.HTMLURL,
			access:      *team.Permission,
		})
	}
	return teams, nil
}

// repoTeamsResult holds the result of getting teams for a repository
type repoTeamsResult struct {
	repoName string
	teams    []teamInfo
	err      error
}

// teamMatchResult represents the result of team matching analysis
type teamMatchResult struct {
	exactMatches []teamInfo      // teams with access to ALL repositories
	closeMatches []teamMatchInfo // teams with access to over half of repositories
}

// teamMatchInfo contains team information along with match statistics
type teamMatchInfo struct {
	team          teamInfo
	accessCount   int      // number of repositories this team has access to
	accessPercent float64  // percentage of repositories this team has access to
	missingRepos  []string // repositories this team doesn't have access to
}

// findTeamsWithAccessToAllRepos finds teams that have access to all specified repositories
// It processes repositories in parallel for efficiency by launching separate goroutines
// for each repository to fetch team information concurrently.
// Returns a slice of teamInfo structs representing teams that have access to ALL repositories,
// or an empty slice if no such teams exist.
func findTeamsWithAccessToAllRepos(ctx context.Context, client *github.Client, owner string, repoNames []string) ([]teamInfo, error) {
	result, err := findTeamsWithAccessAnalysis(ctx, client, owner, repoNames)
	if err != nil {
		return nil, err
	}
	return result.exactMatches, nil
}

// findTeamsWithAccessAnalysis finds teams that have access to repositories with detailed analysis
// It processes repositories in parallel and returns both exact matches (all repos) and close matches (>50% of repos)
func findTeamsWithAccessAnalysis(ctx context.Context, client *github.Client, owner string, repoNames []string) (*teamMatchResult, error) {
	if len(repoNames) == 0 {
		return nil, fmt.Errorf("no repository names provided")
	}

	// Channel to collect results from goroutines
	resultChan := make(chan repoTeamsResult, len(repoNames))
	var wg sync.WaitGroup

	// Launch goroutines to get teams for each repository in parallel
	for _, repoName := range repoNames {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			teams, err := getRepositoryTeams(ctx, client, owner, repo)
			resultChan <- repoTeamsResult{
				repoName: repo,
				teams:    teams,
				err:      err,
			}
		}(repoName)
	}

	// Wait for all goroutines to complete and close the channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	repoTeamsMap := make(map[string][]teamInfo)
	var errors []string

	for result := range resultChan {
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("Error getting teams for repo %s: %v", result.repoName, result.err))
			continue
		}
		repoTeamsMap[result.repoName] = result.teams
	}

	// If we had errors getting teams for some repos, report them
	if len(errors) > 0 {
		log.Printf("Errors occurred while getting teams for some repositories:")
		for _, err := range errors {
			log.Printf("  %s", err)
		}
	}

	// If we couldn't get teams for any repository, return error
	if len(repoTeamsMap) == 0 {
		return nil, fmt.Errorf("could not get teams for any of the specified repositories")
	}

	// Find teams that appear in ALL repositories
	teamAccessCount := make(map[string]int)
	teamDetails := make(map[string]teamInfo)

	// Count how many repositories each team has access to
	for repoName, teams := range repoTeamsMap {
		log.Printf("Repository %s has %d teams with access", repoName, len(teams))
		for _, team := range teams {
			teamAccessCount[team.slug]++
			// Store team details (all repos should have same team details for same slug)
			teamDetails[team.slug] = team
		}
	}

	// Analyze team access patterns
	totalRepos := len(repoTeamsMap)

	var exactMatches []teamInfo
	var closeMatches []teamMatchInfo

	for teamSlug, count := range teamAccessCount {
		team := teamDetails[teamSlug]
		accessPercent := float64(count) / float64(totalRepos) * 100

		if count == totalRepos {
			// Exact match - has access to ALL repositories
			exactMatches = append(exactMatches, team)
		} else if accessPercent > 50.0 {
			// Close match - has access to more than 50% of repositories
			missingRepos := findMissingRepos(teamSlug, repoTeamsMap)
			closeMatches = append(closeMatches, teamMatchInfo{
				team:          team,
				accessCount:   count,
				accessPercent: accessPercent,
				missingRepos:  missingRepos,
			})
		}
	}

	return &teamMatchResult{
		exactMatches: exactMatches,
		closeMatches: closeMatches,
	}, nil
}

// findMissingRepos identifies which repositories a team doesn't have access to
func findMissingRepos(teamSlug string, repoTeamsMap map[string][]teamInfo) []string {
	var missingRepos []string

	for repoName, teams := range repoTeamsMap {
		hasAccess := false
		for _, team := range teams {
			if team.slug == teamSlug {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			missingRepos = append(missingRepos, repoName)
		}
	}

	return missingRepos
}

// findAndReportTeamsWithAccessToAllRepos is a convenience function that finds teams
// with access to all repos and reports the results, including close matches
func findAndReportTeamsWithAccessToAllRepos(ctx context.Context, client *github.Client, owner string, repoNames []string) {
	fmt.Printf("Analyzing team access patterns for %d repositories...\n", len(repoNames))
	fmt.Printf("Repositories: %v\n\n", repoNames)

	result, err := findTeamsWithAccessAnalysis(ctx, client, owner, repoNames)
	if err != nil {
		log.Printf("Error finding teams: %v", err)
		return
	}

	// Report exact matches (100% access)
	if len(result.exactMatches) > 0 {
		fmt.Printf("🎯 EXACT MATCHES - Teams with access to ALL %d repositories:\n\n", len(repoNames))
		for i, team := range result.exactMatches {
			fmt.Printf("%d. Team: %s\n", i+1, team.name)
			fmt.Printf("   Slug: %s\n", team.slug)
			if team.description != "" {
				fmt.Printf("   Description: %s\n", team.description)
			}
			fmt.Printf("   Access Level: %s\n", team.access)
			fmt.Printf("   URL: %s\n", team.url)
			fmt.Printf("   Coverage: 100%% (%d/%d repositories)\n", len(repoNames), len(repoNames))
			fmt.Printf("\n")
		}
	} else {
		fmt.Printf("🎯 EXACT MATCHES: No teams found with access to ALL repositories.\n\n")
	}

	// Report close matches (>50% access)
	if len(result.closeMatches) > 0 {
		fmt.Printf("🔍 CLOSE MATCHES - Teams with access to more than half of the repositories:\n\n")
		for i, match := range result.closeMatches {
			fmt.Printf("%d. Team: %s\n", i+1, match.team.name)
			fmt.Printf("   Slug: %s\n", match.team.slug)
			if match.team.description != "" {
				fmt.Printf("   Description: %s\n", match.team.description)
			}
			fmt.Printf("   Access Level: %s\n", match.team.access)
			fmt.Printf("   URL: %s\n", match.team.url)
			fmt.Printf("   Coverage: %.1f%% (%d/%d repositories)\n", match.accessPercent, match.accessCount, len(repoNames))
			if len(match.missingRepos) > 0 {
				fmt.Printf("   Missing access to: %v\n", match.missingRepos)
			}
			fmt.Printf("\n")
		}
	} else {
		fmt.Printf("🔍 CLOSE MATCHES: No teams found with access to more than half of the repositories.\n\n")
	}

	// Summary
	totalMatches := len(result.exactMatches) + len(result.closeMatches)
	if totalMatches == 0 {
		fmt.Printf("📊 SUMMARY: No teams found with significant access coverage.\n")
		fmt.Printf("To find teams with access to individual repositories, use the --teams flag with each repository name.\n")
	} else {
		fmt.Printf("📊 SUMMARY: Found %d exact matches and %d close matches.\n", len(result.exactMatches), len(result.closeMatches))
	}
}

// addUserAsRepoCollaborator adds a user as a repository collaborator with admin permission
// It checks for existing admin collaborators and warns if they were added recently or should be removed
func addUserAsRepoCollaborator(ctx context.Context, client *github.Client, owner, repo, username string) error {
	log.Printf("Checking existing collaborators for repository %s/%s", owner, repo)

	// List all collaborators with admin permission
	opts := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Affiliation: "direct", // Only direct collaborators, not team members
	}

	collaborators, _, err := client.Repositories.ListCollaborators(ctx, owner, repo, opts)
	if err != nil {
		return fmt.Errorf("failed to list collaborators: %w", err)
	}

	// Check for existing admin collaborators
	now := time.Now()
	hasOldAdmin := false

	for _, collab := range collaborators {
		// Check if this collaborator has admin permission
		if collab.Permissions != nil && collab.Permissions["admin"] {
			// Skip if it's the user we're trying to add
			if *collab.Login == username {
				// Get the invitation/permission details to check when it was added
				permission, _, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, username)
				if err == nil && permission.Permission != nil {
					log.Printf("⚠️  User %s already has %s access to repository %s/%s", username, *permission.Permission, owner, repo)

					// Try to get invitation info to determine how long ago
					invitations, _, invErr := client.Repositories.ListInvitations(ctx, owner, repo, nil)
					if invErr == nil {
						for _, inv := range invitations {
							if *inv.Invitee.Login == username {
								duration := now.Sub(inv.CreatedAt.Time)
								if duration.Hours() < 24 {
									fmt.Printf("⚠️  WARNING: User %s already has admin access (added %.1f hours ago)\n", username, duration.Hours())
									return nil
								}
							}
						}
					}

					// If we couldn't determine the time, just warn they already have access
					fmt.Printf("ℹ️  User %s already has admin access to this repository\n", username)
					return nil
				}
				continue
			}

			// Another user has admin access
			log.Printf("Found existing admin collaborator: %s", *collab.Login)

			// Try to determine when they were added
			// Note: GitHub API doesn't directly provide "added date" for collaborators
			// We can check invitations for pending ones, but for accepted ones we need to check events
			invitations, _, invErr := client.Repositories.ListInvitations(ctx, owner, repo, nil)
			addedTime := time.Time{}

			if invErr == nil {
				for _, inv := range invitations {
					if inv.Invitee != nil && *inv.Invitee.Login == *collab.Login {
						addedTime = inv.CreatedAt.Time
						break
					}
				}
			}

			// If we couldn't find invitation, check recent events
			if addedTime.IsZero() {
				events, _, evErr := client.Activity.ListRepositoryEvents(ctx, owner, repo, &github.ListOptions{PerPage: 100})
				if evErr == nil {
					for _, event := range events {
						if *event.Type == "MemberEvent" {
							payload := event.Payload().(*github.MemberEvent)
							if payload.Member != nil && *payload.Member.Login == *collab.Login {
								addedTime = *event.CreatedAt
								break
							}
						}
					}
				}
			}

			if !addedTime.IsZero() {
				duration := now.Sub(addedTime)
				if duration.Hours() > 24 {
					fmt.Printf("⚠️  WARNING: User %s has admin access and was added %.1f hours ago (>24h) - should be removed\n",
						*collab.Login, duration.Hours())
					hasOldAdmin = true
				} else {
					fmt.Printf("⚠️  WARNING: User %s already has admin access (added %.1f hours ago)\n",
						*collab.Login, duration.Hours())
				}
			} else {
				// Can't determine when they were added, just warn
				fmt.Printf("⚠️  WARNING: User %s has admin access (added date unknown) - consider reviewing\n", *collab.Login)
			}
		}
	}

	// Add the user as collaborator with admin permission
	log.Printf("Adding user %s as admin collaborator to repository %s/%s", username, owner, repo)

	opts2 := &github.RepositoryAddCollaboratorOptions{
		Permission: "admin",
	}

	_, resp, err := client.Repositories.AddCollaborator(ctx, owner, repo, username, opts2)
	if err != nil {
		return fmt.Errorf("failed to add collaborator: %w", err)
	}

	if resp.StatusCode == 204 {
		fmt.Printf("✅ User %s already had admin access to repository %s/%s\n", username, owner, repo)
	} else if resp.StatusCode == 201 {
		fmt.Printf("✅ Successfully sent admin collaboration invitation to user %s for repository %s/%s\n", username, owner, repo)
	} else {
		fmt.Printf("✅ Updated permissions for user %s on repository %s/%s\n", username, owner, repo)
	}

	if hasOldAdmin {
		fmt.Printf("\n💡 TIP: Consider removing admin users that were added more than 24 hours ago\n")
	}

	return nil
}

// listRepositoryCollaborators lists all collaborators on a repository with their permissions and when they were added
func listRepositoryCollaborators(ctx context.Context, client *github.Client, owner, repo string) error {
	log.Printf("Fetching collaborators for repository %s/%s", owner, repo)

	// List all collaborators
	opts := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Affiliation: "direct", // Only direct collaborators, not team members
	}

	collaborators, _, err := client.Repositories.ListCollaborators(ctx, owner, repo, opts)
	if err != nil {
		return fmt.Errorf("failed to list collaborators: %w", err)
	}

	if len(collaborators) == 0 {
		fmt.Printf("📋 No direct collaborators found for repository %s/%s\n", owner, repo)
		fmt.Printf("   (Note: Team members are not included in this list)\n")
		return nil
	}

	fmt.Printf("📋 Collaborators for repository %s/%s:\n\n", owner, repo)

	// Get invitations to help determine when collaborators were added
	invitations, _, _ := client.Repositories.ListInvitations(ctx, owner, repo, nil)
	invitationMap := make(map[string]*github.RepositoryInvitation)
	for _, inv := range invitations {
		if inv.Invitee != nil {
			invitationMap[*inv.Invitee.Login] = inv
		}
	}

	// Get repository events to find when members were added
	events, _, _ := client.Activity.ListRepositoryEvents(ctx, owner, repo, &github.ListOptions{PerPage: 100})
	eventMap := make(map[string]time.Time)
	if events != nil {
		for _, event := range events {
			if *event.Type == "MemberEvent" {
				payload := event.Payload().(*github.MemberEvent)
				if payload.Member != nil && payload.Action != nil && *payload.Action == "added" {
					login := *payload.Member.Login
					if _, exists := eventMap[login]; !exists {
						eventMap[login] = *event.CreatedAt
					}
				}
			}
		}
	}

	now := time.Now()

	for i, collab := range collaborators {
		fmt.Printf("%d. 👤 User: %s\n", i+1, *collab.Login)

		// Determine permission level
		var permissions []string
		if collab.Permissions != nil {
			if collab.Permissions["admin"] {
				permissions = append(permissions, "admin")
			}
			if collab.Permissions["maintain"] {
				permissions = append(permissions, "maintain")
			}
			if collab.Permissions["push"] {
				permissions = append(permissions, "push")
			}
			if collab.Permissions["triage"] {
				permissions = append(permissions, "triage")
			}
			if collab.Permissions["pull"] {
				permissions = append(permissions, "pull")
			}
		}

		if len(permissions) > 0 {
			fmt.Printf("   🔐 Permissions: %s\n", strings.Join(permissions, ", "))
		}

		// Get detailed permission level
		permission, _, permErr := client.Repositories.GetPermissionLevel(ctx, owner, repo, *collab.Login)
		if permErr == nil && permission.Permission != nil {
			fmt.Printf("   📊 Access Level: %s\n", *permission.Permission)
		}

		// Try to determine when they were added
		var addedTime time.Time
		var addedSource string

		// Check invitations first
		if inv, exists := invitationMap[*collab.Login]; exists {
			addedTime = inv.CreatedAt.Time
			addedSource = "invitation"
		}

		// Check events if not found in invitations
		if addedTime.IsZero() {
			if eventTime, exists := eventMap[*collab.Login]; exists {
				addedTime = eventTime
				addedSource = "event"
			}
		}

		if !addedTime.IsZero() {
			duration := now.Sub(addedTime)
			fmt.Printf("   📅 Added: %s (%.1f hours ago)\n", addedTime.Format("2006-01-02 15:04:05"), duration.Hours())
			if addedSource == "invitation" {
				fmt.Printf("   ℹ️  Status: Invitation pending\n")
			}

			// Warn if admin access is old
			if collab.Permissions != nil && collab.Permissions["admin"] && duration.Hours() > 24 {
				fmt.Printf("   ⚠️  WARNING: Admin access granted >24 hours ago - consider reviewing\n")
			}
		} else {
			fmt.Printf("   📅 Added: Unknown (not found in recent events)\n")
		}

		if collab.HTMLURL != nil {
			fmt.Printf("   🔗 Profile: %s\n", *collab.HTMLURL)
		}

		fmt.Printf("\n")
	}

	fmt.Printf("📊 Total: %d direct collaborator(s)\n", len(collaborators))

	return nil
}

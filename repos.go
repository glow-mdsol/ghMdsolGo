package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

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

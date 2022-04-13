package main

import (
	"github.com/google/go-github/v43/github"
	"golang.org/x/net/context"
	"log"
)

type repositoryInfo struct {
	owner        string
	name         string
	description  string
	teams        []string
	templateRepo string
}

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

	var repository *github.Repository
	repository = &github.Repository{
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

// Design: docs/contributing/github-integration.md -- latest deployment identity, never fallback success.

package github

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
)

// LatestPagesDeployment reads only the newest github-pages deployment and its
// newest status. A known failure is returned as data for the site owner to
// judge; missing or unknown data returns an error. It never publishes anything.
func (client *Client) LatestPagesDeployment(ctx context.Context) (PagesDeployment, error) {
	result, err := client.latestPages(ctx)
	if err != nil {
		return result, opererr.Wrap(err, "GitHub Pages deployment evidence is incomplete",
			"The current website deployment cannot be bound to a reviewed artifact.", "./le help site check")
	}
	return result, nil
}

func (client *Client) latestPages(ctx context.Context) (PagesDeployment, error) {
	read, cancel, err := client.start(ctx)
	if err != nil {
		return PagesDeployment{}, err
	}
	defer cancel()
	var deployments []struct {
		ID          uint64 `json:"id"`
		SHA         string `json:"sha"`
		Ref         string `json:"ref"`
		Environment string `json:"environment"`
	}
	if err := read.get("deployments", url.Values{"environment": {"github-pages"}, apiPageSizeKey: {"1"}}, &deployments); err != nil {
		return PagesDeployment{}, err
	}
	if len(deployments) != 1 {
		return PagesDeployment{}, errors.New("API did not return exactly the newest Pages deployment")
	}
	deployment := deployments[0]
	result := PagesDeployment{ID: deployment.ID, SHA: deployment.SHA, Ref: deployment.Ref, Environment: deployment.Environment}
	if deployment.ID == 0 {
		return result, errors.New("pages deployment omitted a positive ID")
	}
	if _, err := fullRevision(deployment.SHA); err != nil {
		return result, errors.New("pages deployment omitted its full commit ID")
	}
	if deployment.Ref != "gh-pages" {
		return result, errors.New("latest Pages deployment does not name gh-pages")
	}
	if deployment.Environment != "github-pages" {
		return result, errors.New("latest deployment is not the github-pages environment")
	}
	var statuses []struct {
		ID             uint64 `json:"id"`
		State          string `json:"state"`
		EnvironmentURL string `json:"environment_url"`
		LogURL         string `json:"log_url"`
		DeploymentURL  string `json:"deployment_url"`
	}
	deploymentPath := "deployments/" + strconv.FormatUint(deployment.ID, 10)
	if err := read.get(deploymentPath+"/statuses", url.Values{apiPageSizeKey: {"1"}}, &statuses); err != nil {
		return result, err
	}
	if len(statuses) != 1 {
		return result, errors.New("pages deployment has no single newest status")
	}
	status := statuses[0]
	result.StatusID, result.Status = status.ID, readable(status.State)
	result.EnvironmentURL, result.LogURL = status.EnvironmentURL, status.LogURL
	if status.ID == 0 {
		return result, errors.New("pages status omitted a positive identity")
	}
	if !slices.Contains(client.states.Deployment, status.State) {
		return result, errors.New("pages deployment status is unknown")
	}
	if status.DeploymentURL != "" {
		if status.DeploymentURL != apiRoot+deploymentPath {
			return result, errors.New("pages status belongs to another deployment")
		}
	}
	if status.EnvironmentURL != "https://ze-software.net/" {
		if status.EnvironmentURL != "https://ze-software.net" {
			return result, errors.New("pages deployment names a different website origin")
		}
	}
	if status.LogURL != "" {
		parsed, err := url.Parse(status.LogURL)
		if err != nil {
			return result, errors.New("pages log URL is malformed")
		}
		if parsed.Scheme != apiSecureScheme {
			return result, errors.New("pages log URL is not HTTPS")
		}
		if parsed.Host != "github.com" {
			return result, errors.New("pages log URL leaves GitHub")
		}
		if parsed.User != nil {
			return result, errors.New("pages log URL contains credentials")
		}
	}
	return result, nil
}

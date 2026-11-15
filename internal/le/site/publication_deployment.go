// Design: docs/contributing/gh-pages.md -- live confirmation is evidence for one committed artifact.
// Related: publication.go -- reviewed hashes and bounded file identities.
package site

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	githubreader "github.com/ze-software/ze/internal/le/github"
)

const publicationSiteURL = "https://ze-software.net/"

// The transport seam changes no origin or credential policy. Tests answer the
// fixed GitHub and public-site hosts without contacting either service.
var publicationTransport http.RoundTripper = http.DefaultTransport

func confirmPublication(ctx context.Context, paths Paths, receipt publicationReceipt, expected string) (publicationReport, error) {
	if err := publicationValidateReceipt(receipt); err != nil {
		return publicationReport{}, err
	}
	if err := publicationCommit(ctx, paths, receipt, expected); err != nil {
		return publicationReport{}, err
	}
	client, err := githubreader.NewClient(publicationTransport)
	if err != nil {
		return publicationReport{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	deployment, err := client.LatestPagesDeployment(ctx)
	if err != nil {
		return publicationReport{}, err
	}
	if deployment.SHA != expected {
		return publicationReport{}, errors.New("the newest Pages deployment names another artifact commit")
	}
	if deployment.Ref != "gh-pages" {
		return publicationReport{}, errors.New("the newest Pages deployment is not from gh-pages")
	}
	if deployment.Environment != "github-pages" {
		return publicationReport{}, errors.New("the newest deployment names another environment")
	}
	if deployment.Status != "success" {
		return publicationReport{}, fmt.Errorf("the newest Pages deployment is %s, not success", deployment.Status)
	}
	if strings.TrimSuffix(deployment.EnvironmentURL, "/") != strings.TrimSuffix(publicationSiteURL, "/") {
		return publicationReport{}, errors.New("Pages reports another public-site origin")
	}
	probed, err := publicationProbe(ctx, receipt, expected)
	if err != nil {
		return publicationReport{}, err
	}
	// Bind the end of confirmation too: another session may have replaced the
	// artifact while the bounded network reads were in progress.
	if _, err := checkPublication(ctx, paths); err != nil {
		return publicationReport{}, err
	}
	if err := publicationCommit(ctx, paths, receipt, expected); err != nil {
		return publicationReport{}, err
	}
	report := publicationAnswer(paths, receipt, "confirmed")
	report.ArtifactRevision = expected
	report.Probed = probed
	return report, nil
}

func publicationProbe(ctx context.Context, receipt publicationReceipt, expected string) (int, error) {
	known := make(map[string]publicationFile, len(receipt.Files))
	for _, file := range receipt.Files {
		known[file.Path] = file
	}
	names := append([]string(nil), receipt.Changed...)
	if _, present := known["index.html"]; present {
		found := false
		for _, name := range names {
			if name == "index.html" {
				found = true
			}
		}
		if !found {
			names = append(names, "index.html")
		}
	}
	client := &http.Client{Transport: publicationTransport, Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("publication probes do not follow redirects")
		}}
	probed := 0
	// Both lists were validated against the fixed artifact file-count bound.
	for _, name := range names {
		address, served, err := publicationAddress(name, expected)
		if err != nil {
			return probed, err
		}
		if !served {
			continue
		}
		file, present := known[name]
		if !present {
			return probed, fmt.Errorf("changed live path %s has no reviewed file", name)
		}
		if err := publicationProbeFile(ctx, client, address, file); err != nil {
			return probed, err
		}
		probed++
	}
	for _, name := range receipt.Deleted {
		address, served, err := publicationAddress(name, expected)
		if err != nil {
			return probed, err
		}
		if !served {
			continue
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
		if err != nil {
			return probed, err
		}
		response, err := client.Do(request)
		if err != nil {
			return probed, fmt.Errorf("check deleted live path %s: %w", name, err)
		}
		closeErr := response.Body.Close()
		if closeErr != nil {
			return probed, closeErr
		}
		if response.StatusCode != http.StatusNotFound {
			if response.StatusCode != http.StatusGone {
				return probed, fmt.Errorf("deleted live path %s answered HTTP %d, not 404 or 410", name, response.StatusCode)
			}
		}
		probed++
	}
	if probed == 0 {
		return 0, errors.New("the receipt contains no public route or asset to confirm")
	}
	return probed, nil
}

// publicationAddress's served guard excludes GitHub control files; their bytes
// remain part of the commit/receipt identity, but Pages does not serve them.
func publicationAddress(name, expected string) (address string, served bool, err error) {
	if err := publicationPath(name); err != nil {
		return "", false, err
	}
	if name == "CNAME" {
		return "", false, nil
	}
	if name == artifactIgnoreFile {
		return "", false, nil
	}
	if name == ".nojekyll" {
		return "", false, nil
	}
	path := "/" + name
	if name == "index.html" {
		path = "/"
	} else if prefix, index := strings.CutSuffix(name, "/index.html"); index {
		path = "/" + prefix + "/"
	}
	parsed := url.URL{Scheme: "https", Host: "ze-software.net", Path: path}
	query := url.Values{}
	query.Set("ze-publication", expected)
	parsed.RawQuery = query.Encode()
	return parsed.String(), true, nil
}

func publicationProbeFile(ctx context.Context, client *http.Client, address string, file publicationFile) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("read live path %s: %w", file.Path, err)
	}
	defer response.Body.Close() //nolint:errcheck // the bounded content read below determines the probe verdict
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("live path %s answered HTTP %d", file.Path, response.StatusCode)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(response.Body, file.Size+1))
	if err != nil {
		return fmt.Errorf("read live path %s: %w", file.Path, err)
	}
	if size != file.Size {
		return fmt.Errorf("live path %s has another byte length", file.Path)
	}
	if hex.EncodeToString(hash.Sum(nil)) != file.Digest {
		return fmt.Errorf("live path %s differs from the reviewed bytes", file.Path)
	}
	return nil
}

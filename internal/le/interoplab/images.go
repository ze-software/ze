// Design: docs/architecture/testing/interop.md -- selected scenario image preparation.
// Related: lab.go -- suite ownership and dynamic peer preparation.
package interoplab

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// peerImageBuilds selects declarations from the final peer plans, not from
// scenario names or container roles. A dynamically prepared peer MUST name its
// image declaration, just like a static peer; an undeclared image is an error.
func peerImageBuilds(peers []PeerConfig, declarations []ImageBuild) ([]ImageBuild, error) {
	needed := make(map[string]bool, len(peers))
	for index := range peers {
		needed[peers[index].Image] = false
	}
	builds := make([]ImageBuild, 0, len(needed))
	for index := range declarations {
		build := &declarations[index]
		found, used := needed[build.Name]
		if !used {
			continue
		}
		if found {
			return nil, fmt.Errorf("suite repeats image declaration %q", build.Name)
		}
		needed[build.Name] = true
		builds = append(builds, *build)
	}
	for index := range peers {
		peer := &peers[index]
		if !needed[peer.Image] {
			return nil, fmt.Errorf("peer %q needs undeclared image %q; add its image declaration to the suite", peer.Name, peer.Image)
		}
	}
	return builds, nil
}

// preparePeerImages retains each successful build once for the suite. The
// caller MUST release every appended result after all scenarios finish, even
// when preparation fails. Failed optional images fail only their dependents;
// they never resolve to an old cached tag and never trigger a second attempt.
func (s Suite) preparePeerImages(ctx context.Context, peers []PeerConfig, references map[string]string, images *[]ImageResult) error {
	builds, err := peerImageBuilds(peers, s.Images)
	if err != nil {
		return err
	}
	pending := builds[:0]
	for index := range builds {
		build := &builds[index]
		attempted := false
		for _, image := range *images {
			if image.Name != build.Name {
				continue
			}
			if image.Error != "" {
				return fmt.Errorf("image %q preparation failed: %s", image.Name, image.Error)
			}
			attempted = true
			break
		}
		if !attempted {
			pending = append(pending, *build)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	var line textbuf.Buffer
	s.progress(line.Str("interop: preparing ").Int(int64(len(pending))).Str(" selected image(s)"))
	results, resolved, err := s.prepareImages(ctx, pending)
	*images = append(*images, results...)
	for _, image := range results {
		if image.Error == "" {
			references[image.Name] = resolved[image.Name]
		}
	}
	for _, image := range results {
		if image.Error != "" {
			return fmt.Errorf("image %q preparation failed: %s", image.Name, image.Error)
		}
	}
	return err
}

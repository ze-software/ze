// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: kernelcheck.go -- the kernel probe, the first container run under a Ze profile
// Related: register_apparmor.go -- the kernel probe's registration
// Related: ../setup/dockerkernel_apparmor.go -- the action that loads a registered profile
//
// A lab container that needs more than Docker's docker-default AppArmor
// profile grants runs under a Ze profile that grants exactly that, never
// unconfined (owner decision D-7). Each profile registers here, so the load
// action, the refusal and the container start all read one declaration.

package interoplab

import (
	"context"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// AppArmorLoadCommandPrefix is the load action's command without the profile
// name it confirms.
const AppArmorLoadCommandPrefix = "./le setup docker-kernel apparmor confirm "

// AppArmorProfile is one Ze AppArmor profile a lab container runs under.
type AppArmorProfile struct {
	// Name is the profile's name, as `--security-opt apparmor=` selects it.
	Name string
	// Runs names what runs under it, for a refusal: "the kernel probe".
	Runs string
	// Needs names what docker-default denies it, for a refusal.
	Needs string
	// Text answers the profile's source, ready for apparmor_parser.
	Text func() string
	// Next is the command to run once the profile is loaded.
	Next string
}

// appArmorRegistry holds every registered profile by name. Written only by
// RegisterAppArmorProfile from init, so it is read without a lock.
var appArmorRegistry = map[string]AppArmorProfile{}

// RegisterAppArmorProfile records profile. It MUST be called from init in a
// register file; a profile with no name or text, or a second one under a
// name, is a Ze defect.
func RegisterAppArmorProfile(profile AppArmorProfile) {
	if profile.Name == "" {
		panic("BUG: an AppArmor profile registered without a name")
	}
	if profile.Text == nil {
		panic("BUG: AppArmor profile " + profile.Name + " registered without a text")
	}
	if profile.Next == "" {
		panic("BUG: AppArmor profile " + profile.Name + " registered without the command that follows its load")
	}
	if _, taken := appArmorRegistry[profile.Name]; taken {
		panic("BUG: AppArmor profile " + profile.Name + " registered twice")
	}
	appArmorRegistry[profile.Name] = profile
}

// LookupAppArmorProfile answers the profile registered under name.
func LookupAppArmorProfile(name string) (AppArmorProfile, bool) {
	profile, found := appArmorRegistry[name]
	return profile, found
}

// AppArmorProfileNames answers every registered profile's name, sorted.
func AppArmorProfileNames() []string {
	names := make([]string, 0, len(appArmorRegistry))
	for name := range appArmorRegistry {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// AppArmorLoadCommandFor answers the command that loads the named profile.
func AppArmorLoadCommandFor(name string) string {
	return AppArmorLoadCommandPrefix + name
}

// appArmorSecurityOption answers the docker run arguments that confine a
// container under the named profile: none on a daemon that applies no
// AppArmor, the option when the host has the profile loaded, and a refusal
// naming the load command when it has not, or when no lab registered it.
func (d *Docker) appArmorSecurityOption(ctx context.Context, name string) ([]string, error) {
	profile, registered := LookupAppArmorProfile(name)
	if !registered {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("a lab container names AppArmor profile ").Str(name).
			Str(", which no lab registers").String())
	}
	applies, err := dockerAppArmor(ctx, d)
	if err != nil {
		return nil, err
	}
	if !applies {
		return nil, nil
	}
	if missing := labAppArmorProfileMissing(profile); missing != nil {
		return nil, missing
	}
	return []string{"--security-opt", "apparmor=" + name}, nil
}

// labAppArmorProfileMissing answers the refusal for a Linux host whose kernel
// lists its loaded profiles and lacks profile, or nil. An unreadable list is
// no answer either way, so the container runs and a daemon that cannot apply
// the profile refuses it through Docker's own error, which names it.
func labAppArmorProfileMissing(profile AppArmorProfile) error {
	profiles, readable := appArmorProfiles()
	if !readable {
		return nil
	}
	if appArmorProfileLoaded(profiles, profile.Name) {
		return nil
	}
	var refusal textbuf.Buffer
	return errors.New(refusal.Str("the Docker daemon applies AppArmor, and ").Str(profile.Runs).
		Str(" runs under Ze's profile ").Str(profile.Name).Str(", which ").Str(appArmorProfilesPath).
		Str(" does not list. Docker's docker-default profile denies ").Str(profile.Needs).
		Str(", so load Ze's: ").Str(AppArmorLoadCommandFor(profile.Name)).String())
}

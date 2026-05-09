// Design: plan/spec-appliance-1-builder.md — full image build (assemble + gok + ext4)

package appliance

import (
	"flag"
	"fmt"
	"os"
)

func init() {
	cmdBuild = runBuild
}

func runBuild(args []string) int {
	fs := flag.NewFlagSet("appliance build", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Build all appliances in the directory")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance build [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *allFlag {
		return buildAll()
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	return buildOne(fs.Arg(0))
}

func buildOne(name string) int {
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var passphrase []byte
	if IsEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	dbPath := DatabasePath(dir, name)
	if code := assembleZeFS(dir, name, cfg, passphrase, dbPath); code != exitOK {
		return code
	}
	defer os.Remove(dbPath) //nolint:errcheck // cleanup after build

	gokPath := "bin/gok"
	if _, err := os.Stat(gokPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s not found (run: make bin/gok)\n", gokPath)
		os.Remove(dbPath) //nolint:errcheck // cleanup
		return exitError
	}

	ts := ImageTimestamp()
	imgName := ImageFileName(ts)
	imgPath := AppliancePath(dir, name) + "/" + imgName

	// gok invocation + ext4 inject would happen here
	fmt.Fprintf(os.Stderr, "error: gok image build not yet implemented (assemble succeeded, ZeFS at %s)\n", dbPath)

	seedConfig, _ := resolveSeedConfig(dir, name, cfg)
	manifest := &BuildManifest{
		Appliance:  name,
		Timestamp:  ts,
		ZeVersion:  "dev",
		Arch:       cfg.Image.Arch,
		ConfigHash: ConfigHash(seedConfig),
		Image:      imgName,
	}

	manifestPath := AppliancePath(dir, name) + "/build.json"
	if writeErr := WriteManifest(manifestPath, manifest); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: write manifest: %v\n", writeErr)
	}

	_ = imgPath // will be used when gok invocation is implemented
	return exitError
}

func buildAll() int {
	dir := getBaseDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", dir, err)
		return exitError
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_shared" || e.Name()[0] == '.' {
			continue
		}
		if _, loadErr := LoadConfig(ConfigPath(dir, e.Name())); loadErr == nil {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "no appliances found in %s\n", dir)
		return exitError
	}

	succeeded, failed := 0, 0
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "building %s...\n", name)
		if code := buildOne(name); code != exitOK {
			fmt.Fprintf(os.Stderr, "FAILED: %s\n", name)
			failed++
		} else {
			succeeded++
		}
	}

	fmt.Printf("%d succeeded, %d failed\n", succeeded, failed)
	if failed > 0 {
		return exitError
	}
	return exitOK
}

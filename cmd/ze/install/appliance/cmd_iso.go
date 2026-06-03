// Design: plan/spec-install-8-appliance-iso.md — bootable appliance ISO installer

package appliance

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/thirdparty/fat"
)

const (
	defaultISOInitrd = "tools/installer-initrd/build/initrd.img.gz"
	isoBuildTimeout  = 10 * time.Minute
	isoMediaIDBytes  = 16
)

var isoTargetRe = regexp.MustCompile(`^/dev/(sd[a-z]+|vd[a-z]+|xvd[a-z]+|hd[a-z]+|nvme[0-9]+n[0-9]+|mmcblk[0-9]+)$`)

var isoImageNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

const (
	isoGRUBTargetAMD64 = "x86_64-efi"
	isoGRUBTargetARM64 = "arm64-efi"
)

var (
	isoLookPathFn = exec.LookPath
	isoBuilderFn  = runISOBuilder
)

type isoOptions struct {
	imageFile   string
	outputPath  string
	kernelPath  string
	initrdPath  string
	target      string
	builderPath string
	keepStaging bool
}

type isoBuildInput struct {
	appliance   string
	arch        string
	appDir      string
	imagePath   string
	imageName   string
	imageSHA    string
	mediaID     string
	outputPath  string
	kernelPath  string
	initrdPath  string
	target      string
	grubPath    string
	xorrisoPath string
}

type isoManifest struct {
	Appliance        string `json:"appliance"`
	Arch             string `json:"arch"`
	Image            string `json:"image"`
	ImageSHA256      string `json:"image-sha256"`
	ImageCompression string `json:"image-compression,omitempty"`
	Kernel           string `json:"kernel"`
	Initrd           string `json:"initrd"`
	Target           string `json:"target,omitempty"`
	MediaID          string `json:"media-id"`
	CreatedAt        string `json:"created-at"`
}

type isoBuilderCall struct {
	GRUBPath    string
	XorrisoPath string
	OutputPath  string
	StagingDir  string
	GRUBTarget  string
	EFIBootFile string
}

func init() {
	cmdIso = runIso
}

func runIso(args []string) int {
	opts := isoOptions{}
	fs := flag.NewFlagSet("appliance iso", flag.ContinueOnError)
	fs.StringVar(&opts.imageFile, "image", "", "Specific image file inside the appliance directory (default: most recent)")
	fs.StringVar(&opts.outputPath, "output", "", "ISO output path (default: selected image name with .iso in appliance directory)")
	fs.StringVar(&opts.kernelPath, "kernel", "", "Installer kernel path (default: tools/installer-kernel/build/Image)")
	fs.StringVar(&opts.initrdPath, "initrd", defaultISOInitrd, "Installer initrd path")
	fs.StringVar(&opts.target, "target", "", "Optional explicit installer target disk, for example /dev/vda")
	fs.StringVar(&opts.builderPath, "builder", "", "GRUB standalone builder binary (default: grub-mkstandalone or grub2-mkstandalone from PATH)")
	fs.BoolVar(&opts.keepStaging, "keep-staging", false, "Keep temporary ISO staging directory for inspection")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze install appliance iso [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Build a bootable installer ISO around an existing appliance image.\n")
		fmt.Fprintf(os.Stderr, "The selected image must stay inside the appliance directory and must have a matching .sha256 sidecar.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze install appliance iso lab\n")
		fmt.Fprintf(os.Stderr, "  ze install appliance iso --image ze-20260601-120000.img lab\n")
		fmt.Fprintf(os.Stderr, "  ze install appliance iso --target /dev/vda lab\n")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name>\n")
		fs.Usage()
		return exitError
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", fs.Arg(1))
		fs.Usage()
		return exitError
	}

	out, err := buildISO(fs.Arg(0), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	fmt.Fprintf(os.Stdout, "iso ready: %s\n", out) //nolint:errcheck // CLI output
	return exitOK
}

func buildISO(name string, opts isoOptions) (string, error) {
	input, err := resolveISOInput(name, opts)
	if err != nil {
		return "", err
	}

	img, err := os.Open(input.imagePath) //nolint:gosec // path validated by resolveImagePath
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	defer img.Close() //nolint:errcheck // read-only
	if err := verifyImageChecksumRequired(img, input.imagePath, input.imageSHA); err != nil {
		return "", err
	}

	staging, err := os.MkdirTemp(input.appDir, ".iso-staging-*")
	if err != nil {
		return "", fmt.Errorf("create ISO staging: %w", err)
	}
	cleanup := !opts.keepStaging
	defer func() {
		if cleanup {
			os.RemoveAll(staging) //nolint:errcheck // cleanup best effort
		}
	}()

	if err := stageISO(input, img, staging); err != nil {
		return "", err
	}

	builderOutput, err := allocateISOBuilderOutput(input.outputPath)
	if err != nil {
		return "", err
	}
	defer os.Remove(builderOutput) //nolint:errcheck // remove abandoned temp output

	grubTarget, efiBootFile, err := isoGRUBTarget(input.arch)
	if err != nil {
		return "", err
	}
	call := isoBuilderCall{
		GRUBPath:    input.grubPath,
		XorrisoPath: input.xorrisoPath,
		OutputPath:  builderOutput,
		StagingDir:  staging,
		GRUBTarget:  grubTarget,
		EFIBootFile: efiBootFile,
	}
	if err := isoBuilderFn(call); err != nil {
		return "", fmt.Errorf("build ISO: %w", err)
	}
	if err := os.Rename(builderOutput, input.outputPath); err != nil {
		return "", fmt.Errorf("move ISO into place: %w", err)
	}
	return input.outputPath, nil
}

func resolveISOInput(name string, opts isoOptions) (isoBuildInput, error) {
	if !validNameRe.MatchString(name) {
		return isoBuildInput{}, fmt.Errorf("invalid appliance name %q: must match %s", name, validNameRe.String())
	}
	if opts.imageFile != "" && filepath.Base(opts.imageFile) != opts.imageFile {
		return isoBuildInput{}, fmt.Errorf("image %q must be a file name inside the appliance directory", opts.imageFile)
	}
	if opts.imageFile != "" && !isoImageNameRe.MatchString(opts.imageFile) {
		return isoBuildInput{}, fmt.Errorf("image %q must use only [a-zA-Z0-9._-] to remain bootable from ISO media", opts.imageFile)
	}
	if err := validateISOTarget(opts.target); err != nil {
		return isoBuildInput{}, err
	}

	dir := getBaseDir()
	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		return isoBuildInput{}, err
	}
	if err := cfg.Validate(); err != nil {
		return isoBuildInput{}, err
	}

	imgPath, err := resolveImagePath(dir, name, opts.imageFile)
	if err != nil {
		return isoBuildInput{}, err
	}
	if !isoImageNameRe.MatchString(filepath.Base(imgPath)) {
		return isoBuildInput{}, fmt.Errorf("image %q must use only [a-zA-Z0-9._-] to remain bootable from ISO media", filepath.Base(imgPath))
	}
	sha, err := readRequiredImageChecksum(imgPath)
	if err != nil {
		return isoBuildInput{}, err
	}

	kernelPath := opts.kernelPath
	if kernelPath == "" {
		kernelPath = defaultISOKernelPath()
	}
	kernel, err := resolveISOArtifact(kernelPath, "installer kernel", "build the installer kernel under tools/installer-kernel or pass --kernel")
	if err != nil {
		return isoBuildInput{}, err
	}
	initrd, err := resolveISOArtifact(opts.initrdPath, "installer initrd", "make -C tools/installer-initrd or pass --initrd")
	if err != nil {
		return isoBuildInput{}, err
	}
	grubPath, xorrisoPath, err := resolveISOBuilder(opts.builderPath)
	if err != nil {
		return isoBuildInput{}, err
	}
	mediaID, err := generateISOMediaID()
	if err != nil {
		return isoBuildInput{}, err
	}

	appDir := AppliancePath(dir, name)
	out, err := resolveISOOutput(appDir, imgPath, opts.outputPath)
	if err != nil {
		return isoBuildInput{}, err
	}

	return isoBuildInput{
		appliance:   name,
		arch:        cfg.Image.Arch,
		appDir:      appDir,
		imagePath:   imgPath,
		imageName:   filepath.Base(imgPath),
		imageSHA:    sha,
		mediaID:     mediaID,
		outputPath:  out,
		kernelPath:  kernel,
		initrdPath:  initrd,
		target:      opts.target,
		grubPath:    grubPath,
		xorrisoPath: xorrisoPath,
	}, nil
}

func validateISOTarget(target string) error {
	if target == "" {
		return nil
	}
	if !isoTargetRe.MatchString(target) {
		return fmt.Errorf("target %q must be a whole disk path such as /dev/sda, /dev/vda, /dev/nvme0n1, or /dev/mmcblk0", target)
	}
	return nil
}

func resolveISOArtifact(path, label, hint string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s path is required", label)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%s not found at %s; %s", label, path, hint)
	}
	if st.IsDir() {
		return "", fmt.Errorf("%s %s is a directory", label, path)
	}
	return abs, nil
}

func defaultISOKernelPath() string {
	return filepath.Join("tools", "installer-kernel", "build", "Image")
}

func resolveISOBuilder(builder string) (grubPath, xorrisoPath string, err error) {
	if builder != "" {
		grubPath, err = resolveExecutable(builder)
		if err != nil {
			return "", "", err
		}
	} else {
		var firstErr error
		for _, candidate := range []string{"grub-mkstandalone", "grub2-mkstandalone"} {
			grubPath, err = isoLookPathFn(candidate)
			if err == nil {
				break
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if grubPath == "" {
			if firstErr != nil {
				return "", "", errors.New("grub-mkstandalone not found; install GRUB EFI tooling or pass --builder")
			}
			return "", "", errors.New("grub-mkstandalone not found")
		}
	}
	xorrisoPath, err = resolveExecutable("xorriso")
	if err != nil {
		return "", "", errors.New("xorriso not found; install xorriso to build installer ISOs")
	}
	return grubPath, xorrisoPath, nil
}

func resolveExecutable(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		abs, err := filepath.Abs(name)
		if err != nil {
			return "", fmt.Errorf("resolve executable: %w", err)
		}
		st, statErr := os.Stat(abs)
		if statErr != nil {
			return "", fmt.Errorf("executable %s not found", name)
		}
		if st.IsDir() || st.Mode()&0o111 == 0 {
			return "", fmt.Errorf("executable %s is not executable", name)
		}
		return abs, nil
	}
	resolved, err := isoLookPathFn(name)
	if err != nil {
		return "", fmt.Errorf("executable %s not found", name)
	}
	return resolved, nil
}

func isoGRUBTarget(arch string) (target, efiBootFile string, err error) {
	switch arch {
	case archAMD64:
		return isoGRUBTargetAMD64, "BOOTX64.EFI", nil
	case archARM64:
		return isoGRUBTargetARM64, "BOOTAA64.EFI", nil
	default:
		return "", "", fmt.Errorf("unsupported appliance ISO architecture %q", arch)
	}
}

func resolveISOOutput(appDir, imgPath, output string) (string, error) {
	if output == "" {
		stem := strings.TrimSuffix(filepath.Base(imgPath), filepath.Ext(imgPath))
		output = filepath.Join(appDir, stem+".iso")
	}
	abs, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}
	parent := filepath.Dir(abs)
	st, statErr := os.Stat(parent)
	if statErr != nil {
		return "", fmt.Errorf("output directory %s not found", parent)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("output parent %s is not a directory", parent)
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("resolve output directory %s: %w", parent, err)
	}
	realOutput := filepath.Join(realParent, filepath.Base(abs))
	if realOutput == imgPath {
		return "", errors.New("output path must not overwrite the selected image")
	}
	if outInfo, err := os.Stat(abs); err == nil {
		if outInfo.IsDir() {
			return "", fmt.Errorf("output path %s is a directory", abs)
		}
		imgInfo, imgErr := os.Stat(imgPath)
		if imgErr == nil && os.SameFile(outInfo, imgInfo) {
			return "", errors.New("output path must not overwrite the selected image")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat output path %s: %w", abs, err)
	}
	return abs, nil
}

func generateISOMediaID() (string, error) {
	var raw [isoMediaIDBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate ISO media id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func allocateISOBuilderOutput(finalPath string) (string, error) {
	tmp, err := os.CreateTemp(filepath.Dir(finalPath), "."+filepath.Base(finalPath)+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temporary ISO output: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary ISO output: %w", err)
	}
	if err := os.Remove(tmpPath); err != nil {
		return "", fmt.Errorf("prepare temporary ISO output: %w", err)
	}
	return tmpPath, nil
}

func readRequiredImageChecksum(imgPath string) (string, error) {
	checksumPath := imgPath + ".sha256"
	data, err := os.ReadFile(checksumPath) //nolint:gosec // appliance sidecar
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("checksum sidecar %s is required; run `ze install appliance build <name>` to create it", filepath.Base(checksumPath))
		}
		return "", fmt.Errorf("read checksum sidecar: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || !isSHA256Hex(fields[0]) {
		return "", fmt.Errorf("checksum sidecar %s does not contain a valid SHA-256", filepath.Base(checksumPath))
	}
	return strings.ToLower(fields[0]), nil
}

func isSHA256Hex(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func verifyImageChecksumRequired(f *os.File, imgPath, expectedHex string) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek image for checksum: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("read image for checksum: %w", err)
	}
	actualHex := hex.EncodeToString(h.Sum(nil))
	if actualHex != expectedHex {
		return fmt.Errorf("checksum mismatch for %s (expected %s..., got %s...)", filepath.Base(imgPath), expectedHex[:12], actualHex[:12])
	}
	return nil
}

func stageISO(input isoBuildInput, img *os.File, staging string) error {
	for _, dir := range []string{
		filepath.Join(staging, "boot", "grub"),
		filepath.Join(staging, "ze-install", "images"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create staging directory: %w", err)
		}
	}
	if err := copyFile(input.kernelPath, filepath.Join(staging, "boot", "kernel")); err != nil {
		return err
	}
	if err := copyFile(input.initrdPath, filepath.Join(staging, "boot", "initrd.img.gz")); err != nil {
		return err
	}
	compressedName := input.imageName + ".gz"
	imageDest := filepath.Join(staging, "ze-install", "images", compressedName)
	compressedSHA, err := compressOpenFileGzip(img, imageDest)
	if err != nil {
		return err
	}
	checksumLine := compressedSHA + "  " + compressedName + "\n"
	if err := os.WriteFile(imageDest+".sha256", []byte(checksumLine), 0o644); err != nil { //nolint:gosec // checksum metadata
		return fmt.Errorf("write staged checksum: %w", err)
	}
	manifest := isoManifest{
		Appliance:        input.appliance,
		Arch:             input.arch,
		Image:            compressedName,
		ImageSHA256:      input.imageSHA,
		ImageCompression: "gzip",
		Kernel:           "boot/kernel",
		Initrd:           "boot/initrd.img.gz",
		Target:           input.target,
		MediaID:          input.mediaID,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	manifestData, err := json.MarshalIndent(&manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ISO manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(staging, "ze-install", "manifest.json"), manifestData, 0o644); err != nil { //nolint:gosec // metadata
		return fmt.Errorf("write ISO manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "ze-install", "media-id"), []byte(input.mediaID+"\n"), 0o644); err != nil { //nolint:gosec // metadata
		return fmt.Errorf("write ISO media id: %w", err)
	}
	input.imageName = compressedName
	return writeGrubConfig(filepath.Join(staging, "boot", "grub", "grub.cfg"), input)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // controlled source artifact
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close() //nolint:errcheck // read-only
	return copyOpenFile(in, dst)
}

func copyOpenFile(src *os.File, dst string) error {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek %s: %w", src.Name(), err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // staged artifact
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close() //nolint:errcheck // close after copy
	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	return nil
}

func compressOpenFileGzip(src *os.File, dst string) (string, error) {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek %s: %w", src.Name(), err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // staged artifact
	if err != nil {
		return "", fmt.Errorf("create %s: %w", dst, err)
	}
	closeOut := true
	defer func() {
		if closeOut {
			_ = out.Close()
		}
	}()
	h := sha256.New()
	w := io.MultiWriter(out, h)
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("create gzip writer: %w", err)
	}
	if _, err := io.Copy(gz, src); err != nil {
		return "", fmt.Errorf("compress %s: %w", src.Name(), err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("finalize gzip: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", dst, err)
	}
	closeOut = false
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeGrubConfig(path string, input isoBuildInput) error {
	consoleArgs, err := isoKernelConsoleArgs(input.arch)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("set timeout=5\n")
	b.WriteString("set default=0\n")
	b.WriteString("search --no-floppy --file /ze-install/media-id --set=root\n")
	b.WriteString("menuentry 'Install Ze appliance ")
	b.WriteString(input.appliance)
	b.WriteString("' {\n")
	b.WriteString("    linux /boot/kernel ")
	b.WriteString(consoleArgs)
	b.WriteString(" ze.source=iso ze.image=")
	b.WriteString(input.imageName)
	b.WriteString(" ze.media-id=")
	b.WriteString(input.mediaID)
	if input.target != "" {
		b.WriteString(" ze.target=")
		b.WriteString(input.target)
	}
	b.WriteString("\n")
	b.WriteString("    initrd /boot/initrd.img.gz\n")
	b.WriteString("}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // boot config
}

func isoKernelConsoleArgs(arch string) (string, error) {
	switch arch {
	case archAMD64:
		return "console=tty0 console=ttyS0", nil
	case archARM64:
		return "console=tty0 console=ttyAMA0", nil
	default:
		return "", fmt.Errorf("unsupported appliance ISO architecture %q", arch)
	}
}

func runISOBuilder(call isoBuilderCall) error {
	ctx, cancel := context.WithTimeout(context.Background(), isoBuildTimeout)
	defer cancel()

	cfgPath := filepath.Join(call.StagingDir, "boot", "grub", "grub.cfg")
	efiDir := filepath.Join(call.StagingDir, "EFI", "BOOT")
	if err := os.MkdirAll(efiDir, 0o750); err != nil {
		return fmt.Errorf("create EFI staging: %w", err)
	}
	efiBinaryPath := filepath.Join(efiDir, call.EFIBootFile)
	grubCmd := exec.CommandContext(ctx, call.GRUBPath, "-O", call.GRUBTarget, "-o", efiBinaryPath, "boot/grub/grub.cfg="+cfgPath) //nolint:gosec // controlled argv
	grubCmd.Stdout = os.Stderr
	grubCmd.Stderr = os.Stderr
	if err := grubCmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("GRUB EFI builder timed out")
		}
		return err
	}

	efiImagePath := filepath.Join(efiDir, "efiboot.img")
	if err := createEFIBootImage(efiImagePath, call.EFIBootFile, efiBinaryPath); err != nil {
		return err
	}

	xorrisoCmd := exec.CommandContext(ctx, call.XorrisoPath, "-as", "mkisofs", "-R", "-J", "-o", call.OutputPath, "-eltorito-alt-boot", "-e", "EFI/BOOT/efiboot.img", "-no-emul-boot", call.StagingDir) //nolint:gosec // controlled argv
	xorrisoCmd.Stdout = os.Stderr
	xorrisoCmd.Stderr = os.Stderr
	if err := xorrisoCmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("xorriso timed out")
		}
		return err
	}
	return nil
}

func createEFIBootImage(imagePath, efiBootFile, efiBinaryPath string) error {
	out, err := os.OpenFile(imagePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // staged artifact
	if err != nil {
		return fmt.Errorf("create EFI boot image: %w", err)
	}
	closeOut := true
	defer func() {
		if closeOut {
			_ = out.Close()
		}
	}()

	fw, err := fat.NewWriter(out)
	if err != nil {
		return fmt.Errorf("create FAT writer: %w", err)
	}
	modTime := time.Now().UTC()
	if err := fw.Mkdir(filepath.Join("EFI", "BOOT"), modTime); err != nil {
		return fmt.Errorf("create EFI boot directories: %w", err)
	}
	entry, err := fw.File(filepath.Join("EFI", "BOOT", efiBootFile), modTime)
	if err != nil {
		return fmt.Errorf("add EFI boot file: %w", err)
	}
	in, err := os.Open(efiBinaryPath) //nolint:gosec // staged artifact
	if err != nil {
		return fmt.Errorf("open EFI binary: %w", err)
	}
	defer in.Close() //nolint:errcheck // read-only
	if _, err := io.Copy(entry, in); err != nil {
		return fmt.Errorf("write EFI boot file: %w", err)
	}
	if err := fw.Flush(); err != nil {
		return fmt.Errorf("finalize EFI boot image: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close EFI boot image: %w", err)
	}
	closeOut = false

	img, err := os.Open(imagePath) //nolint:gosec // staged artifact
	if err != nil {
		return fmt.Errorf("reopen EFI boot image: %w", err)
	}
	defer img.Close() //nolint:errcheck // read-only
	rd, err := fat.NewReader(img)
	if err != nil {
		return fmt.Errorf("verify EFI boot image: %w", err)
	}
	if _, _, err := rd.Extents("/" + filepath.ToSlash(filepath.Join("EFI", "BOOT", efiBootFile))); err != nil {
		return fmt.Errorf("verify EFI boot file in image: %w", err)
	}
	return nil
}

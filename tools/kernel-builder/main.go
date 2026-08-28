package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

func main() {
	fs := flag.NewFlagSet("ze-kernel-builder", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	version := fs.String("version", os.Getenv("KERNEL_VERSION"), "Linux kernel version")
	arch := fs.String("arch", envOr("ARCH", "arm64"), "target architecture")
	profile := fs.String("profile", envOr("PROFILE", "qemu"), "kernel profile")
	jobs := fs.String("jobs", os.Getenv("JOBS"), "parallel kernel build jobs")
	sourceDir := fs.String("src-dir", envOr("SRC_DIR", "/src"), "config source directory")
	outputDir := fs.String("out-dir", envOr("OUT_DIR", "/out"), "output directory")
	modules := fs.String("modules", envOr("MODULES", "no"), "install runtime modules")
	patchesDir := fs.String("patches-dir", os.Getenv("PATCHES_DIR"), "patch series directory")
	firmwareDir := fs.String("firmware-dir", os.Getenv("FIRMWARE_DIR"), "firmware directory")
	var fragments stringList
	fs.Var(&fragments, "fragment", "config fragment in merge order")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *version == "" {
		fmt.Fprintln(os.Stderr, "FATAL: --version or KERNEL_VERSION is required")
		os.Exit(2)
	}
	err := kernelbuilder.RunWorker(context.Background(), kernelbuilder.WorkerRequest{
		Version: *version, Arch: *arch, Profile: *profile, Jobs: *jobs,
		SourceDir: *sourceDir, OutputDir: *outputDir, Modules: *modules,
		PatchesDir: *patchesDir, FirmwareDir: *firmwareDir, Fragments: fragments,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(2)
	}
}

type stringList []string

func (values *stringList) String() string         { return fmt.Sprint([]string(*values)) }
func (values *stringList) Set(value string) error { *values = append(*values, value); return nil }
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

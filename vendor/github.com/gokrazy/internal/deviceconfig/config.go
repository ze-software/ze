// Package deviceconfig contains any device-specific configuration.
package deviceconfig

// RootFile represents a file that is stored on a raw disk device.
type RootFile struct {
	// Name of the file to be read from kernel package.
	// It is also the name of the update device handler (`/update/device-specific/<Name>`).
	Name string
	// Offset on root disk device where this file/blob should be stored.
	Offset int64
	// Maximum length of file to be accepted during updates.
	// `[Offset, Offset+MaxLength)` must not overlap for any 2 RootFiles for a device.
	MaxLength int64
}

type DeviceConfig struct {
	// Does the device not support GPT. If true, only emit MBR partition table.
	MBROnlyWithoutGPT bool
	// Bootloader files stored on raw root disk device.
	RootDeviceFiles []RootFile
	// Slug is a unique, short string used by gokr-packer to refer to this device.
	Slug string
	// Override the default boot partition start LBA (default: 8192 sectors = 4MiB)
	BootPartitionStartLBA int64
}

const (
	sectorSize                         = 512
	DefaultBootPartitionStartLBA int64 = 8192
)

var (
	// DeviceConfigs contains a mapping from device model (github.com/gokrazy/gokrazy.Model) to device-specific config
	DeviceConfigs = map[string]DeviceConfig{
		// Odroid HC1/HC2/XU4
		// (https://github.com/torvalds/linux/blob/c9e6606c7fe92b50a02ce51dda82586ebdf99b48/arch/arm/boot/dts/exynos5422-odroidhc1.dts#L14)
		"Hardkernel Odroid HC1": {
			MBROnlyWithoutGPT: true,
			// https://wiki.odroid.com/odroid-xu4/software/partition_table#tab__odroid-xu341
			RootDeviceFiles: []RootFile{
				{"bl1.bin", 1 * sectorSize, 30 * sectorSize},       // sectors 1 - 30
				{"bl2.bin", 31 * sectorSize, 32 * sectorSize},      // sectors 31 - 62
				{"u-boot.bin", 63 * sectorSize, 1440 * sectorSize}, // sectors 63 - 1502
				{"tzsw.bin", 1503 * sectorSize, 512 * sectorSize},  // sectors 1503 - 2014
			},
			Slug: "odroidhc1",
		},
		"QEMU testing MBR": {
			MBROnlyWithoutGPT: true,
			Slug:              "qemumbrtesting",
		},
		"Pine64 Rock64": {
			// https://opensource.rock-chips.com/wiki_Partitions
			RootDeviceFiles: []RootFile{
				// u-boot-rockchip.bin is an überpackage that include TPL, SPL, U-Boot and u-boot.dtb.
				// u-boot can build it as a single file with `make rock64-rk3328_defconfig && make u-boot-rockchip.bin`
				// and its easier to work with instead of dealing with separate files.
				{"u-boot-rockchip.bin", 64 * sectorSize, 32704 * sectorSize}, // sectors 64 - 32768
			},
			BootPartitionStartLBA: 32768, // 16MiB from start of disk
			Slug:                  "rock64",
		},
		"FriendlyElec NanoPi Neo": {
			// https://linux-sunxi.org/Bootable_SD_card
			MBROnlyWithoutGPT: true,
			RootDeviceFiles: []RootFile{
				// u-boot-sunxi-with-spl.bin is an überpackage that include TPL, SPL, U-Boot and u-boot.dtb.
				// u-boot can build it as a single file with `make nanopi_neo_defconfig && make u-boot-sunxi-with-spl.bin`
				// and its easier to work with instead of dealing with separate files.
				{"u-boot-sunxi-with-spl.bin", 16 * sectorSize, 2032 * sectorSize}, // sectors 16 - 2048
			},
			BootPartitionStartLBA: 2048, // 1MiB from start of disk
			Slug:                  "nanopi_neo",
		},
	}
)

func GetDeviceConfigBySlug(slug string) (DeviceConfig, bool) {
	for _, cfg := range DeviceConfigs {
		if cfg.Slug == slug {
			return cfg, true
		}
	}

	return DeviceConfig{}, false
}

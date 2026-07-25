package system_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
)

func TestConsoleConfigParse(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	console := sys.GetOrCreateContainer("console")
	dev := config.NewTree()
	dev.Set("speed", "115200")
	console.AddListEntry("device", "ttyS0", dev)

	sc := system.ExtractSystemConfig(tree)
	if assert.Len(t, sc.ConsoleDevices, 1) {
		assert.Equal(t, "ttyS0", sc.ConsoleDevices[0].Name)
		assert.Equal(t, 115200, sc.ConsoleDevices[0].Speed)
	}
}

func TestConsoleConfigParse_DefaultSpeed(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	console := sys.GetOrCreateContainer("console")
	dev := config.NewTree()
	console.AddListEntry("device", "ttyS0", dev)

	sc := system.ExtractSystemConfig(tree)
	if assert.Len(t, sc.ConsoleDevices, 1) {
		assert.Equal(t, "ttyS0", sc.ConsoleDevices[0].Name)
		assert.Equal(t, 115200, sc.ConsoleDevices[0].Speed, "default speed should be 115200")
	}
}

func TestConsoleConfigParse_AllSpeeds(t *testing.T) {
	for _, speed := range []string{"9600", "19200", "38400", "57600", "115200"} {
		t.Run(speed, func(t *testing.T) {
			tree := config.NewTree()
			sys := tree.GetOrCreateContainer("system")
			console := sys.GetOrCreateContainer("console")
			dev := config.NewTree()
			dev.Set("speed", speed)
			console.AddListEntry("device", "ttyS0", dev)

			sc := system.ExtractSystemConfig(tree)
			if assert.Len(t, sc.ConsoleDevices, 1) {
				assert.NotZero(t, sc.ConsoleDevices[0].Speed)
			}
		})
	}
}

func TestConsoleConfigParse_InvalidSpeed(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	console := sys.GetOrCreateContainer("console")
	dev := config.NewTree()
	dev.Set("speed", "12345")
	console.AddListEntry("device", "ttyS0", dev)

	sc := system.ExtractSystemConfig(tree)
	if assert.Len(t, sc.ConsoleDevices, 1) {
		assert.Equal(t, 115200, sc.ConsoleDevices[0].Speed, "invalid speed should fall back to 115200")
	}
}

func TestConsoleConfigParse_MultipleDevices(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	console := sys.GetOrCreateContainer("console")

	dev0 := config.NewTree()
	dev0.Set("speed", "115200")
	console.AddListEntry("device", "ttyS0", dev0)

	dev1 := config.NewTree()
	dev1.Set("speed", "9600")
	console.AddListEntry("device", "ttyS1", dev1)

	sc := system.ExtractSystemConfig(tree)
	assert.Len(t, sc.ConsoleDevices, 2)

	speeds := map[string]int{}
	for _, d := range sc.ConsoleDevices {
		speeds[d.Name] = d.Speed
	}
	assert.Equal(t, 115200, speeds["ttyS0"])
	assert.Equal(t, 9600, speeds["ttyS1"])
}

func TestConsoleConfigParse_NoConsoleBlock(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system")

	sc := system.ExtractSystemConfig(tree)
	assert.Empty(t, sc.ConsoleDevices)
}

func TestExtractConsoleFromMap(t *testing.T) {
	tree := map[string]any{
		"system": map[string]any{
			"console": map[string]any{
				"device": map[string]any{
					"ttyS0": map[string]any{
						"speed": "115200",
					},
					"ttyS1": map[string]any{
						"speed": "9600",
					},
				},
			},
		},
	}

	devices := system.ExtractConsoleFromMap(tree)
	assert.Len(t, devices, 2)

	speeds := map[string]int{}
	for _, d := range devices {
		speeds[d.Name] = d.Speed
	}
	assert.Equal(t, 115200, speeds["ttyS0"])
	assert.Equal(t, 9600, speeds["ttyS1"])
}

func TestExtractConsoleFromMap_Empty(t *testing.T) {
	tree := map[string]any{}
	assert.Empty(t, system.ExtractConsoleFromMap(tree))

	tree2 := map[string]any{"system": map[string]any{}}
	assert.Empty(t, system.ExtractConsoleFromMap(tree2))
}

func TestExtractConsoleFromMap_InvalidSpeed(t *testing.T) {
	tree := map[string]any{
		"system": map[string]any{
			"console": map[string]any{
				"device": map[string]any{
					"ttyS0": map[string]any{
						"speed": "99999",
					},
				},
			},
		},
	}

	devices := system.ExtractConsoleFromMap(tree)
	if assert.Len(t, devices, 1) {
		assert.Equal(t, 115200, devices[0].Speed, "invalid speed should default to 115200")
	}
}

func TestConsoleDevicePathTraversal(t *testing.T) {
	tests := []struct {
		name   string
		device string
		valid  bool
	}{
		{"bare name", "ttyS0", true},
		{"bare USB name", "ttyUSB0", true},
		{"path traversal", "../ttyS0", false},
		{"absolute path", "/dev/ttyS0", false},
		{"slash in name", "foo/bar", false},
		{"empty", "", false},
		{"null byte", "ttyS0\x00evil", false},
		{"control char", "ttyS0\n", false},
		{"tab", "tty\tS0", false},
		{"DEL", "ttyS0\x7f", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, system.ValidConsoleDeviceName(tt.device))
		})
	}
}

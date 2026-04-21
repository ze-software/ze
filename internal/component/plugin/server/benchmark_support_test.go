package server

import "codeberg.org/thomas-mangin/ze/internal/component/config/yang"

func internalBuildTestWireToPath() map[string]string {
	loader, _ := yang.DefaultLoader()
	return yang.WireMethodToPath(loader)
}

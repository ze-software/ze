package server

import "github.com/ze-software/ze/internal/component/config/yang"

func internalBuildTestWireToPath() map[string]string {
	loader, _ := yang.DefaultLoader()
	return yang.WireMethodToPath(loader)
}

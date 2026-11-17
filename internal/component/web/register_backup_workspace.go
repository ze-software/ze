// Design: docs/architecture/web-interface.md -- offline backup workspace routes.
package web

func init() {
	// One subtree build owns all requests for its backup workspaces. Separate
	// builders for GET and POST would give each method a different owner.
	RegisterWebRoute(WebRoute{
		Pattern: backupWorkspacePath,
		Wrap:    WrapAuth,
		Build:   HandleBackupWorkspace,
		Portal:  &PortalService{Key: "backups", Title: "Backup workspace", Path: backupWorkspacePath, Direct: true},
	})
}

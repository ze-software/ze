// Design: docs/architecture/fleet-interface.md -- the manage family: configuration, rollouts, audit, templates and administration
// Related: handler_config.go, handler_rollouts.go, handler_audit.go, handler_admin.go -- the builders registered here
// Related: register_tabs.go -- the tabs these pages fill, in their order

package fleetui

func init() {
	registerRolloutFields()
	registerAuditFields()

	MustRegisterPage(Page{Kind: ObjectDevice, Tab: tabDeviceConfig, Content: configContent, SidePanel: configPanel})
	MustRegisterPage(Page{Kind: ObjectEstate, Tab: tabEstateRollouts, Content: rolloutsContent})

	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminAudit, Content: auditContent, SidePanel: auditPanel})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminDevices, Content: adminDevicesContent, SidePanel: adminDevicesPanel})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminLabels, Content: adminLabelsContent})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminTreeViews, Content: adminTreeViewsContent})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminFilters, Content: adminFiltersContent})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminTemplates, Content: adminTemplatesContent, SidePanel: adminTemplatesPanel})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminAccess, Content: adminAccessContent})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminHistory, Content: adminHistoryContent})
	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: tabAdminSettings, Content: adminSettingsContent})

	MustRegisterAction(manageActionName, manageActionAnswer)
	MustRegisterFragment(fragmentConfigState, configStateFragment)
	MustRegisterFragment(fragmentConfigText, configTextFragment)
	MustRegisterFragment(fragmentConfigHistory, configHistoryFragment)
	MustRegisterFragment(fragmentDivergence, divergenceFragment)
	MustRegisterFragment(fragmentRolloutPreview, rolloutPreviewFragment)
}

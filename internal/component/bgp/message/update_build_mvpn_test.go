package message

// BuildMVPN / BuildGroupedMVPN / mvpnNLRISize / writeMVPNNLRI were removed
// by spec-route-config-plugin-migration. MVPN routes now build via the generic
// BuildPlugin path; the NLRI is built by the bgp-nlri-mvpn plugin and route grouping
// is done by the reactor's generic pluginRouteGroupKey. Wire output (including
// shared-join + source-join grouping and NLRI sizing) is covered byte-for-byte by
// test/encode/mvpn.ci and the mvpn plugin's config tests.

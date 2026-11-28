// Design: docs/architecture/isis/isis-4-component-config.md -- fixture registrations for MUP Segment Discovery locator-originator checks

package fixture

func init() {
	Register("plugin/mup-locator-origin-publisher", mupLocatorOriginPublisherDriver)
	Register("plugin/mup-segment-discovery-observer", mupSegmentDiscoveryObserverDriver, StopsDaemon)
}

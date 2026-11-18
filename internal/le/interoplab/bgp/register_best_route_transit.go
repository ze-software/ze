// Design: docs/architecture/testing/interop.md -- ordinary selected BGP transit.
package bgp

const (
	bestRouteTransitFRR        = "bgp-best-route-transit-frr"
	bestRouteTransitBIRD       = "bgp-best-route-transit-bird"
	bestRouteTransitFRRBackup  = "frr-backup"
	bestRouteTransitBIRDBackup = "bird-backup"
	bestRouteTransitBIRDSink   = "bird-sink"
	bestRouteTransitSubject    = "10.245.0.0/24"
	bestRouteTransitControl    = "10.245.1.0/24"
	bestRouteTransitDenied     = "10.245.2.0/24"
)

func init() {
	specialCheckers[bestRouteTransitFRR] = checkBestRouteTransitFRR
	specialCheckers[bestRouteTransitBIRD] = checkBestRouteTransitBIRD
	registerPeerPreparation(bestRouteTransitFRR, prepareBestRouteTransitFRR)
	registerPeerPreparation(bestRouteTransitBIRD, prepareBestRouteTransitBIRD)
}

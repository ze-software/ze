// Design: docs/architecture/api/process-protocol.md -- metrics a plugin publishes.
// Related: ../sdk/metrics.go -- the SDK registry that accumulates and publishes

package rpc

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// MethodMetricsPublish carries a plugin's metric declarations and the changes
// since its previous publish. The plugin chooses when to send it.
const MethodMetricsPublish = "ze-plugin-engine:metrics-publish"

// Bounds of one plugin's metrics, enforced by the SDK and the engine alike.
const (
	MetricsDeclaredMax   = 512     // metric names one plugin may declare
	MetricsSeriesMax     = 4096    // labeled series one plugin may hold across its vectors
	MetricNameMax        = 128     // octets in a metric name
	MetricHelpMax        = 1024    // octets in a help string
	MetricLabelNamesMax  = 16      // label names on one vector
	MetricLabelValueMax  = 256     // octets in one label value
	MetricBucketsMax     = 64      // buckets of one histogram, the implicit +Inf excluded
	MetricBucketCountMax = 1 << 20 // observations one update may add to one bucket
	MetricsPublishMax    = 2 * (MetricsDeclaredMax + MetricsSeriesMax)
	MetricNamePrefix     = "ze_" // every forwarded metric name, so none meets a runtime collector
)

// MetricKind is the Registry method a declaration stands for.
type MetricKind string

const (
	MetricCounter      MetricKind = "counter"
	MetricGauge        MetricKind = "gauge"
	MetricHistogram    MetricKind = "histogram"
	MetricCounterVec   MetricKind = "counter-vec"
	MetricGaugeVec     MetricKind = "gauge-vec"
	MetricHistogramVec MetricKind = "histogram-vec"
)

// MetricDeclaration creates a metric, or names an existing one of the same
// shape. Labels is set for a vector kind only, Buckets for a histogram kind
// only, resolved by the sender (never empty).
type MetricDeclaration struct {
	Name    string     `json:"name"`
	Help    string     `json:"help"`
	Kind    MetricKind `json:"kind"`
	Labels  []string   `json:"labels,omitempty"`
	Buckets []float64  `json:"buckets,omitempty"`
}

// MetricSeries names one series: a metric and, for a vector, its label values
// in declaration order.
type MetricSeries struct {
	Name   string   `json:"name"`
	Values []string `json:"values,omitempty"`
}

// MetricUpdate is the change of one series since the plugin's previous
// publish. A counter's Value is the amount added; a gauge's Value is its
// current value. A histogram carries, for each bucket with the +Inf bucket
// last, the number of observations it gained (Counts) and their sum (Sums).
type MetricUpdate struct {
	Name   string    `json:"name"`
	Values []string  `json:"values,omitempty"`
	Value  float64   `json:"value,omitempty"`
	Counts []uint64  `json:"counts,omitempty"`
	Sums   []float64 `json:"sums,omitempty"`
}

// MetricsPublishInput is one publish. The engine applies the declarations,
// then the deletions, then the updates, so a series deleted and touched again
// since the previous publish ends with only its new value.
type MetricsPublishInput struct {
	Declare []MetricDeclaration `json:"declare,omitempty"`
	Delete  []MetricSeries      `json:"delete,omitempty"`
	Update  []MetricUpdate      `json:"update,omitempty"`
}

// MetricRefusal names one declaration or update the engine did not apply.
// Item is the list it answers: "declare", "delete" or "update".
type MetricRefusal struct {
	Item   string   `json:"item"`
	Name   string   `json:"name"`
	Values []string `json:"values,omitempty"`
	Reason string   `json:"reason"`
}

// MetricsPublishOutput lists what the engine refused; every other item was
// applied. Unavailable means the engine has no metrics registry yet and
// applied nothing, so the plugin keeps its changes and sends them again.
type MetricsPublishOutput struct {
	Unavailable bool            `json:"unavailable,omitempty"`
	Refused     []MetricRefusal `json:"refused,omitempty"`
}

// IsVector reports whether the kind carries labels.
func (k MetricKind) IsVector() bool {
	return k == MetricCounterVec || k == MetricGaugeVec || k == MetricHistogramVec
}

// IsHistogram reports whether the kind carries buckets.
func (k MetricKind) IsHistogram() bool {
	return k == MetricHistogram || k == MetricHistogramVec
}

func (k MetricKind) valid() bool {
	switch k {
	case MetricCounter, MetricGauge, MetricHistogram, MetricCounterVec, MetricGaugeVec, MetricHistogramVec:
		return true
	}
	return false
}

// ValidateMetricsPublish checks the structural bounds of a publish, on both
// sides of the transport. What a name already means in the engine is judged
// there, per item.
func ValidateMetricsPublish(in *MetricsPublishInput) error {
	if len(in.Declare)+len(in.Delete)+len(in.Update) > MetricsPublishMax {
		return fmt.Errorf("metrics publish carries %d items, above the limit of %d", len(in.Declare)+len(in.Delete)+len(in.Update), MetricsPublishMax)
	}
	for i := range in.Declare {
		if err := ValidateMetricDeclaration(&in.Declare[i]); err != nil {
			return err
		}
	}
	for i := range in.Delete {
		if err := validateSeries(in.Delete[i].Name, in.Delete[i].Values); err != nil {
			return err
		}
	}
	for i := range in.Update {
		if err := validateUpdate(&in.Update[i]); err != nil {
			return err
		}
	}
	return nil
}

// ValidateMetricDeclaration refuses a declaration the Prometheus client would
// panic on: a bad name or label name, a duplicate label, labels on a scalar,
// none on a vector, "le" on a histogram, or buckets that do not increase.
func ValidateMetricDeclaration(d *MetricDeclaration) error {
	if err := validateMetricName(d.Name); err != nil {
		return err
	}
	if !d.Kind.valid() {
		return fmt.Errorf("metric %s: unknown kind %q, want counter, gauge, histogram, counter-vec, gauge-vec or histogram-vec", d.Name, d.Kind)
	}
	if d.Help == "" {
		return fmt.Errorf("metric %s: help is empty; describe what it counts", d.Name)
	}
	if len(d.Help) > MetricHelpMax {
		return fmt.Errorf("metric %s: help is %d octets, above the limit of %d", d.Name, len(d.Help), MetricHelpMax)
	}
	if err := validateLabelNames(d); err != nil {
		return err
	}
	return validateBuckets(d)
}

func validateLabelNames(d *MetricDeclaration) error {
	if !d.Kind.IsVector() {
		if len(d.Labels) != 0 {
			return fmt.Errorf("metric %s: a %s carries no labels", d.Name, d.Kind)
		}
		return nil
	}
	if len(d.Labels) == 0 {
		return fmt.Errorf("metric %s: a %s needs at least one label name", d.Name, d.Kind)
	}
	if len(d.Labels) > MetricLabelNamesMax {
		return fmt.Errorf("metric %s: %d label names, above the limit of %d", d.Name, len(d.Labels), MetricLabelNamesMax)
	}
	for i, label := range d.Labels {
		if !labelNameValid(label) {
			return fmt.Errorf("metric %s: label name %q is not [a-zA-Z_][a-zA-Z0-9_]* or starts with __", d.Name, label)
		}
		if d.Kind.IsHistogram() && label == "le" {
			return fmt.Errorf("metric %s: a histogram cannot carry the label le, which Prometheus reserves for buckets", d.Name)
		}
		if slices.Contains(d.Labels[:i], label) {
			return fmt.Errorf("metric %s: label name %s appears twice", d.Name, label)
		}
	}
	return nil
}

func validateBuckets(d *MetricDeclaration) error {
	if !d.Kind.IsHistogram() {
		if len(d.Buckets) != 0 {
			return fmt.Errorf("metric %s: a %s carries no buckets", d.Name, d.Kind)
		}
		return nil
	}
	if len(d.Buckets) == 0 {
		return fmt.Errorf("metric %s: a %s needs its resolved buckets", d.Name, d.Kind)
	}
	if len(d.Buckets) > MetricBucketsMax {
		return fmt.Errorf("metric %s: %d buckets, above the limit of %d", d.Name, len(d.Buckets), MetricBucketsMax)
	}
	for i, bound := range d.Buckets {
		if math.IsNaN(bound) {
			return fmt.Errorf("metric %s: bucket %d is not a number", d.Name, i)
		}
		if i > 0 && bound <= d.Buckets[i-1] {
			return fmt.Errorf("metric %s: buckets must increase, and bucket %d (%v) does not", d.Name, i, bound)
		}
	}
	return nil
}

func validateSeries(name string, values []string) error {
	if err := validateMetricName(name); err != nil {
		return err
	}
	if len(values) > MetricLabelNamesMax {
		return fmt.Errorf("metric %s: %d label values, above the limit of %d", name, len(values), MetricLabelNamesMax)
	}
	for _, value := range values {
		if len(value) > MetricLabelValueMax {
			return fmt.Errorf("metric %s: a label value is %d octets, above the limit of %d", name, len(value), MetricLabelValueMax)
		}
	}
	return nil
}

func validateUpdate(u *MetricUpdate) error {
	if err := validateSeries(u.Name, u.Values); err != nil {
		return err
	}
	if math.IsNaN(u.Value) || math.IsInf(u.Value, 0) {
		return fmt.Errorf("metric %s: value is not finite", u.Name)
	}
	if len(u.Counts) != len(u.Sums) {
		return fmt.Errorf("metric %s: %d bucket counts but %d bucket sums", u.Name, len(u.Counts), len(u.Sums))
	}
	if len(u.Counts) > MetricBucketsMax+1 {
		return fmt.Errorf("metric %s: %d bucket counts, above the limit of %d", u.Name, len(u.Counts), MetricBucketsMax+1)
	}
	for i, count := range u.Counts {
		if count > MetricBucketCountMax {
			return fmt.Errorf("metric %s: bucket %d gains %d observations in one update, above the limit of %d", u.Name, i, count, MetricBucketCountMax)
		}
		if math.IsNaN(u.Sums[i]) || math.IsInf(u.Sums[i], 0) {
			return fmt.Errorf("metric %s: the sum of bucket %d is not finite", u.Name, i)
		}
	}
	return nil
}

func validateMetricName(name string) error {
	if !strings.HasPrefix(name, MetricNamePrefix) {
		return fmt.Errorf("metric name %q does not start with %s, which every Ze metric carries", name, MetricNamePrefix)
	}
	if len(name) > MetricNameMax {
		return fmt.Errorf("metric name %s is %d octets, above the limit of %d", name, len(name), MetricNameMax)
	}
	for i := range len(name) {
		c := name[i]
		if !nameOctet(c) && c != ':' {
			return fmt.Errorf("metric name %q holds %q; a name is [a-zA-Z_:][a-zA-Z0-9_:]*", name, c)
		}
	}
	return nil
}

func labelNameValid(label string) bool {
	if label == "" || strings.HasPrefix(label, "__") {
		return false
	}
	if label[0] >= '0' && label[0] <= '9' {
		return false
	}
	for i := range len(label) {
		if !nameOctet(label[i]) {
			return false
		}
	}
	return true
}

func nameOctet(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

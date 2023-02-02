package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	SnapshotAttemptConcurrentTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "snapshot_attempt_concurrent_requests",
			Help: "Total number of concurrent snapshot attempts",
		},
	)

	SnapshotAttemptDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "snapshot_attempt_duration_seconds",
			Help:    "Snapshot durations from the moment the release PipelineRun was created til the release is marked as finished",
			Buckets: []float64{15, 30, 60, 150, 300, 450, 600, 750, 900, 1050, 1200},
		},
		[]string{"type", "reason"},
	)

	SnapshotAttemptInvalidTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snapshot_attempt_invalid_total",
			Help: "Number of invalid snapshots",
		},
		[]string{"reason"},
	)

	SnapshotRunningSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "integration_snapshot_running_seconds",
			Help:    "Snapshot durations from the moment the buildPipelineRun is completed/ snapshot resource was created til the snapshot is marked as in progress status",
			Buckets: []float64{0.5, 1, 2, 3, 4, 5, 6, 7, 10, 15, 30},
		},
	)

	SnapshotAttemptTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snapshot_attempt_total",
			Help: "Total number of snapshots processed by the operator",
		},
		[]string{"type", "reason"},
	)
)

func RegisterCompletedSnapshot(conditiontype, reason string, startTime metav1.Time, completionTime *metav1.Time) {
	labels := prometheus.Labels{
		"type":   conditiontype,
		"reason": reason,
	}

	SnapshotAttemptConcurrentTotal.Dec()
	SnapshotAttemptDurationSeconds.With(labels).Observe(completionTime.Sub(startTime.Time).Seconds())
	SnapshotAttemptTotal.With(labels).Inc()
}

func RegisterInvalidSnapshot(conditiontype, reason string) {
	//SnapshotAttemptConcurrentTotal.Dec()
	SnapshotAttemptInvalidTotal.With(prometheus.Labels{"reason": reason}).Inc()
	SnapshotAttemptTotal.With(prometheus.Labels{
		"type":   conditiontype,
		"reason": reason,
	}).Inc()
}

func RegisterRunningSnapshot(buildPipelineEnteringTime metav1.Time, inProgressTime *metav1.Time) {
	SnapshotRunningSeconds.Observe(inProgressTime.Sub(buildPipelineEnteringTime.Time).Seconds())
}

func RegisterNewSnapshot() {
	SnapshotAttemptConcurrentTotal.Inc()
}

func init() {
	metrics.Registry.MustRegister(
		SnapshotRunningSeconds,
		SnapshotAttemptConcurrentTotal,
		SnapshotAttemptDurationSeconds,
		SnapshotAttemptInvalidTotal,
		SnapshotAttemptTotal,
	)
}

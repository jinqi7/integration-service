/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Metrics Integration", Ordered, func() {
	BeforeAll(func() {
		metrics.Registry.Unregister(SnapshotRunningSeconds)
		metrics.Registry.Unregister(SnapshotAttemptConcurrentTotal)
		metrics.Registry.Unregister(SnapshotAttemptDurationSeconds)
	})

	var (
		AttemptSnapshotRunningSecondsHeader = inputHeader{
			Name: "snapshot_running_seconds",
			Help: "Integration durations from the moment the buildPipelineRun is completed/ snapshot resource was created til the snapshot is marked as in progress status",
		}
		AttemptSnapshotDurationSecondsHeader = inputHeader{
			Name: "snapshot_attempt_duration_seconds",
			Help: "Snapshot durations from the moment the Snapshot was created til all the Integration PipelineRuns are finished",
		}
		AttemptSnapshotTotalHeader = inputHeader{
			Name: "snapshot_attempt_total",
			Help: "Total number of snapshots processed by the operator",
		}
	)

	Context("When RegisterRunningSnapshot is called", func() {
		// As we need to share metrics within the Context, we need to use "per Context" '(Before|After)All'
		BeforeAll(func() {
			// Mocking metrics to be able to resent data with each tests. Otherwise, we would have to take previous tests into account.
			//
			// 'Help' can't be overridden due to 'https://github.com/prometheus/client_golang/blob/83d56b1144a0c2eb10d399e7abbae3333bebc463/prometheus/registry.go#L314'
			SnapshotRunningSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "snapshot_running_seconds",
					Help:    "Integration durations from the moment the buildPipelineRun is completed/ snapshot resource was created til the snapshot is marked as in progress status",
					Buckets: []float64{1, 5, 10, 30},
				},
			)
			SnapshotAttemptConcurrentTotal = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "snapshot_attempt_concurrent_requests",
					Help: "Total number of concurrent snapshot attempts",
				},
			)
		})

		AfterAll(func() {
			metrics.Registry.Unregister(SnapshotRunningSeconds)
			metrics.Registry.Unregister(SnapshotAttemptConcurrentTotal)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 1, 5, 10 and 30 seconds
		inputSeconds := []float64{1, 3, 8, 15}
		elapsedSeconds := 0.0

		It("increments the 'snapshot_attempt_concurrent_requests'.", func() {
			creationTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				RegisterNewSnapshot()
				startTime := metav1.NewTime(creationTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterRunningSnapshot(creationTime, &startTime)
			}
			Expect(testutil.ToFloat64(SnapshotAttemptConcurrentTotal)).To(Equal(float64(len(inputSeconds))))
		})
		It("registers a new observation for 'snapshot_running_seconds' with the elapsed time from the moment"+
			"the snapshot is created to first integration pipelineRun is started.", func() {
			// Defined buckets for SnapshotRunningSeconds
			timeBuckets := []string{"1", "5", "10", "30"}
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(AttemptSnapshotRunningSecondsHeader, timeBuckets, data, "", elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(SnapshotRunningSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})
	Context("When RegisterCompletedSnapshot is called", func() {
		BeforeAll(func() {
			SnapshotRunningSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "snapshot_running_seconds",
					Help:    "Snapshot durations from the moment the buildPipelineRun is completed/ snapshot resource was created til the snapshot is marked as in progress status",
					Buckets: []float64{1, 5, 10, 30},
				},
			)
			SnapshotAttemptDurationSeconds = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "snapshot_attempt_duration_seconds",
					Help:    "Snapshot durations from the moment the Snapshot was created til all the Integration PipelineRuns are finished",
					Buckets: []float64{60, 600, 1800, 3600},
				},
				[]string{"type", "reason"},
			)

			SnapshotAttemptConcurrentTotal = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "snapshot_attempt_concurrent_requests",
					Help: "Total number of concurrent snapshot attempts",
				},
			)
			SnapshotAttemptTotal = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "snapshot_attempt_total",
					Help: "Total number of snapshots processed by the operator",
				},
				[]string{"type", "reason"},
			)

			//metrics.Registry.MustRegister(SnapshotRunningSeconds, SnapshotAttemptDurationSeconds, SnapshotAttemptConcurrentTotal, SnapshotAttemptTotal)
		})

		AfterAll(func() {
			metrics.Registry.Unregister(SnapshotRunningSeconds)
			metrics.Registry.Unregister(SnapshotAttemptDurationSeconds)
			metrics.Registry.Unregister(SnapshotAttemptConcurrentTotal)
			metrics.Registry.Unregister(SnapshotAttemptTotal)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 60, 600, 1800 and 3600 seconds
		inputSeconds := []float64{30, 500, 1500, 3000}
		elapsedSeconds := 0.0
		labels := fmt.Sprintf(`reason="%s", type="%s",`, "passed", "HACBSIntegrationStatus")

		It("increments 'SnapshotAttemptConcurrentTotal' so we can decrement it to a non-negative number in the next test", func() {
			creationTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				RegisterNewSnapshot()
				startTime := metav1.NewTime(creationTime.Add(time.Second * time.Duration(seconds)))
				RegisterRunningSnapshot(creationTime, &startTime)

			}
			Expect(testutil.ToFloat64(SnapshotAttemptConcurrentTotal)).To(Equal(float64(len(inputSeconds))))
		})

		It("increments 'SnapshotAttemptTotal' and decrements 'SnapshotAttemptConcurrentTotal'", func() {
			startTime := metav1.Time{Time: time.Now()}
			completionTime := startTime
			for _, seconds := range inputSeconds {
				completionTime := metav1.NewTime(completionTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterCompletedSnapshot("HACBSIntegrationStatus", "passed", startTime, &completionTime)
			}
			readerData := createCounterReader(AttemptSnapshotTotalHeader, labels, true, len(inputSeconds))
			Expect(testutil.ToFloat64(SnapshotAttemptConcurrentTotal)).To(Equal(0.0))
			Expect(testutil.CollectAndCompare(SnapshotAttemptTotal, strings.NewReader(readerData))).To(Succeed())
		})

		It("registers a new observation for 'SnapshotAttemptDurationSeconds' with the elapsed time from the moment the Snapshot is created.", func() {
			timeBuckets := []string{"60", "600", "1800", "3600"}
			// For each time bucket how many Snapshot completed below 4 seconds
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(AttemptSnapshotDurationSecondsHeader, timeBuckets, data, labels, elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(SnapshotAttemptDurationSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("When RegisterInvalidSnapshot", func() {
		It("increments the 'SnapshotAttemptInvalidTotal' metric", func() {
			for i := 0; i < 10; i++ {
				RegisterInvalidSnapshot("HACBSIntegrationStatus", "invalid")
			}
			Expect(testutil.ToFloat64(SnapshotAttemptInvalidTotal)).To(Equal(float64(10)))
		})

		It("increments the 'SnapshotAttemptTotal' metric.", func() {
			labels := fmt.Sprintf(`reason="%s", type="%s",`, "invalid", "HACBSIntegrationStatus")
			readerData := createCounterReader(AttemptSnapshotTotalHeader, labels, true, 10.0)
			Expect(testutil.CollectAndCompare(SnapshotAttemptTotal.WithLabelValues("HACBSIntegrationStatus", "invalid"),
				strings.NewReader(readerData))).To(Succeed())
		})
	})
})

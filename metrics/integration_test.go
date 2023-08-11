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
		metrics.Registry.Unregister(SnapshotCreatedToPipelineRunStartedSeconds)
		metrics.Registry.Unregister(SnapshotDurationSeconds)
	})

	var (
		SnapshotPipelineRunStartedSecondsHeader = inputHeader{
			Name: "snapshot_created_to_pipelinerun_started_seconds",
			Help: "Time duration from the moment the snapshot resource was created till a integration pipelineRun is started",
		}
		SnapshotDurationSecondsHeader = inputHeader{
			Name: "snapshot_attempt_duration_seconds",
			Help: "Snapshot durations from the moment the Snapshot was created till the Snapshot is marked as finished",
		}
		SnapshotTotalHeader = inputHeader{
			Name: "snapshot_attempt_total",
			Help: "Total number of snapshots processed by the operator",
		}
	)

	Context("When RegisterPipelineRunStarted is called", func() {
		// As we need to share metrics within the Context, we need to use "per Context" '(Before|After)All'
		BeforeAll(func() {
			// Mocking metrics to be able to resent data with each tests. Otherwise, we would have to take previous tests into account.
			//
			// 'Help' can't be overridden due to 'https://github.com/prometheus/client_golang/blob/83d56b1144a0c2eb10d399e7abbae3333bebc463/prometheus/registry.go#L314'
			SnapshotCreatedToPipelineRunStartedSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "snapshot_created_to_pipelinerun_started_seconds",
					Help:    "Time duration from the moment the snapshot resource was created till a integration pipelineRun is started",
					Buckets: []float64{1, 5, 10, 30},
				},
			)
		})

		AfterAll(func() {
			metrics.Registry.Unregister(SnapshotCreatedToPipelineRunStartedSeconds)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 1, 5, 10 and 30 seconds
		inputSeconds := []float64{1, 3, 8, 15}
		elapsedSeconds := 0.0

		It("registers a new observation for 'snapshot_created_to_pipelinerun_started_seconds' with the elapsed time from the moment"+
			"the snapshot is created to first integration pipelineRun is started.", func() {
			creationTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				startTime := metav1.NewTime(creationTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterPipelineRunStarted(creationTime, &startTime)
			}
			// Defined buckets for SnapshotCreatedToPipelineRunStartedSeconds
			timeBuckets := []string{"1", "5", "10", "30"}
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(SnapshotPipelineRunStartedSecondsHeader, timeBuckets, data, "", elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(SnapshotCreatedToPipelineRunStartedSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("When RegisterCompletedSnapshot is called", func() {
		BeforeAll(func() {
			SnapshotCreatedToPipelineRunStartedSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "snapshot_created_to_pipelinerun_started_seconds",
					Help:    "Snapshot durations from the moment the snapshot resource was created till a integration pipelineRun is started",
					Buckets: []float64{1, 5, 10, 30},
				},
			)

			SnapshotConcurrentTotal.Reset()
			SnapshotDurationSeconds = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "snapshot_attempt_duration_seconds",
					Help:    "Snapshot durations from the moment the Snapshot was created till the Snapshot is marked as finished",
					Buckets: []float64{60, 600, 1800, 3600},
				},
				[]string{"type", "reason"},
			)
			SnapshotTotal = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "snapshot_attempt_total",
					Help: "Total number of snapshots processed by the operator",
				},
				[]string{"type", "reason"},
			)

		})

		AfterAll(func() {
			metrics.Registry.Unregister(SnapshotCreatedToPipelineRunStartedSeconds)
			metrics.Registry.Unregister(SnapshotDurationSeconds)
			SnapshotConcurrentTotal.Reset()
			metrics.Registry.Unregister(SnapshotTotal)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 60, 600, 1800 and 3600 seconds
		inputSeconds := []float64{30, 500, 1500, 3000}
		elapsedSeconds := 0.0
		labels := fmt.Sprintf(`reason="%s", type="%s",`, "passed", "AppStudioIntegrationStatus")

		It("increments 'SnapshotConcurrentTotal' so we can decrement it to a non-negative number in the next test", func() {
			creationTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				RegisterNewSnapshot()
				startTime := metav1.NewTime(creationTime.Add(time.Second * time.Duration(seconds)))
				RegisterPipelineRunStarted(creationTime, &startTime)

			}
			Expect(testutil.ToFloat64(SnapshotConcurrentTotal)).To(Equal(float64(len(inputSeconds))))
		})

		It("increments 'SnapshotTotal' and decrements 'SnapshotConcurrentTotal'", func() {
			startTime := metav1.Time{Time: time.Now()}
			completionTime := startTime
			for _, seconds := range inputSeconds {
				completionTime := metav1.NewTime(completionTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterCompletedSnapshot("AppStudioIntegrationStatus", "passed", startTime, &completionTime)
			}
			readerData := createCounterReader(SnapshotTotalHeader, labels, true, len(inputSeconds))
			Expect(testutil.ToFloat64(SnapshotConcurrentTotal)).To(Equal(0.0))
			Expect(testutil.CollectAndCompare(SnapshotTotal, strings.NewReader(readerData))).To(Succeed())
		})

		It("registers a new observation for 'SnapshotDurationSeconds' with the elapsed time from the moment the Snapshot is created.", func() {
			timeBuckets := []string{"60", "600", "1800", "3600"}
			// For each time bucket how many Snapshot completed below 4 seconds
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(SnapshotDurationSecondsHeader, timeBuckets, data, labels, elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(SnapshotDurationSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("When RegisterInvalidSnapshot", func() {
		It("increments the 'SnapshotInvalidTotal' metric", func() {
			for i := 0; i < 10; i++ {
				RegisterInvalidSnapshot("AppStudioIntegrationStatus", "invalid")
			}
			Expect(testutil.ToFloat64(SnapshotInvalidTotal)).To(Equal(float64(10)))
		})

		It("increments the 'SnapshotTotal' metric.", func() {
			labels := fmt.Sprintf(`reason="%s", type="%s",`, "invalid", "AppStudioIntegrationStatus")
			readerData := createCounterReader(SnapshotTotalHeader, labels, true, 10.0)
			Expect(testutil.CollectAndCompare(SnapshotTotal.WithLabelValues("AppStudioIntegrationStatus", "invalid"),
				strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("when RegisterNewSnapshot is called", func() {
		BeforeEach(func() {
			SnapshotConcurrentTotal.Reset()
		})

		It("increments SnapshotConcurrentTotal", func() {
			Expect(testutil.ToFloat64(SnapshotConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewSnapshot()
			Expect(testutil.ToFloat64(SnapshotConcurrentTotal.WithLabelValues())).To(Equal(float64(1)))
		})
	})
})

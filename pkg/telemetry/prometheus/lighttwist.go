// Copyright 2023 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"fmt"
	"strconv"
	"time"

	"github.com/livekit/mediatransportutil"
	"github.com/livekit/protocol/livekit"
	"github.com/prometheus/client_golang/prometheus"
)

var ntpEpoch = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

type SenderReport struct {
	RtpTimeNormalized uint64
	UtcTime           time.Time
}

var (
	senderReportDelta      *prometheus.GaugeVec
	firstSenderReportDelta *prometheus.GaugeVec

	rtpTimestamp     *prometheus.GaugeVec
	rtpTimestampNorm *prometheus.GaugeVec

	lastSrMap  = make(map[string]SenderReport)
	firstSrMap = make(map[string]SenderReport)
)

func initLightTwistMetrics(nodeID string, nodeType livekit.NodeType) {
	senderReportDelta = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "lighttwist",
		Name:        "delta_sr",
		ConstLabels: prometheus.Labels{"node_id": nodeID, "node_type": nodeType.String()},
	}, []string{"ssrc", "direction", "clock_rate"})

	firstSenderReportDelta = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "lighttwist",
		Name:        "delta_first_sr",
		ConstLabels: prometheus.Labels{"node_id": nodeID, "node_type": nodeType.String()},
	}, []string{"ssrc", "direction", "clock_rate"})

	rtpTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "lighttwist",
		Name:        "rtp_timestamp",
		ConstLabels: prometheus.Labels{"node_id": nodeID, "node_type": nodeType.String()},
	}, []string{"ssrc", "direction", "clock_rate"})

	rtpTimestampNorm = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "lighttwist",
		Name:        "rtp_timestamp_normalized",
		ConstLabels: prometheus.Labels{"node_id": nodeID, "node_type": nodeType.String()},
	}, []string{"ssrc", "direction", "clock_rate"})

	prometheus.MustRegister(senderReportDelta)
	prometheus.MustRegister(firstSenderReportDelta)
	prometheus.MustRegister(rtpTimestamp)
	prometheus.MustRegister(rtpTimestampNorm)
}

func TimestampToUtcTime(timestamp uint64, referenceSenderReport SenderReport) time.Time {
	deltaTimestamp := int64(timestamp) - int64(referenceSenderReport.RtpTimeNormalized)

	// Add the delta converted to nanoseconds (delta * 100 nanoseconds per tick)
	utcTime := referenceSenderReport.UtcTime.Add(time.Duration(deltaTimestamp) * (100 * time.Nanosecond))

	return utcTime
}

func NtpToTime(ntp uint64) time.Time {
	// Extract the seconds and fractional seconds from the NTP timestamp
	seconds := ntp >> 32
	fraction := ntp & 0xFFFFFFFF

	// Convert seconds since NTP epoch to time.Time
	t := ntpEpoch.Add(time.Duration(seconds) * time.Second)

	// Convert fractional seconds to nanoseconds and add to time.Time
	nanoseconds := (fraction * 1e9) >> 32
	t = t.Add(time.Duration(nanoseconds) * time.Nanosecond)

	return t
}

// used for incoming
func SetSenderReportDelta(ssrc string, ntpTime time.Time, rtpTime uint32, clockRate uint32, direction string) {
	timestamp := RtpNormalize100ns(rtpTime, clockRate)

	clockRateLabel := strconv.FormatUint(uint64(clockRate), 10)

	rtpTimestamp.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(rtpTime))
	rtpTimestampNorm.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(timestamp))

	// fmt.Printf("----------------------------\n")
	// fmt.Printf("rtcp.SenderReport %s:\n", direction)
	// fmt.Printf("  ssrc: %s\n", ssrc)
	// fmt.Printf("  rtp_time: %d\n", rtpTime)
	// fmt.Printf("  timestamp: %d\n", timestamp)
	// fmt.Printf("  ntpUtcTime: %s\n", ntpTime)
	// fmt.Printf("  clockRate: %d\n", clockRate)
	// fmt.Printf("----------------------------\n")

	newSenderReport := SenderReport{
		RtpTimeNormalized: timestamp,
		UtcTime:           ntpTime,
	}

	if _, exists := firstSrMap[ssrc]; !exists {
		firstSrMap[ssrc] = newSenderReport
	}

	firstSr := firstSrMap[ssrc]
	firstSrTime := TimestampToUtcTime(firstSr.RtpTimeNormalized, firstSr)
	currSrTime := TimestampToUtcTime(firstSr.RtpTimeNormalized, newSenderReport)

	duration := firstSrTime.Sub(currSrTime)
	deltaMs := duration.Milliseconds()
	firstSenderReportDelta.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(deltaMs))

	if oldSenderReport, exists := lastSrMap[ssrc]; exists {

		newDerivedTime := TimestampToUtcTime(timestamp, newSenderReport)
		oldDerivedTime := TimestampToUtcTime(timestamp, oldSenderReport)

		duration := newDerivedTime.Sub(oldDerivedTime)
		deltaMs := duration.Milliseconds()

		senderReportDelta.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(deltaMs))
	} else {
		senderReportDelta.WithLabelValues(ssrc, direction, clockRateLabel).Set(0.0)
	}

	lastSrMap[ssrc] = newSenderReport
}

// used for outgoing
func SetSenderReportDeltaRaw(ssrc string, ntpTime uint64, rtpTime uint32, clockRate uint32, direction string) {
	timestamp := RtpNormalize100ns(rtpTime, clockRate)

	clockRateLabel := strconv.FormatUint(uint64(clockRate), 10)

	rtpTimestamp.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(rtpTime))
	rtpTimestampNorm.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(timestamp))

	ntpUtcTime := mediatransportutil.NtpTime(ntpTime).Time()

	fmt.Printf("----------------------------\n")
	fmt.Printf("rtcp.SenderReport %s:\n", direction)
	fmt.Printf("  ssrc: %s\n", ssrc)
	fmt.Printf("  rtp_time: %d\n", rtpTime)
	// fmt.Printf("  timestamp: %d\n", timestamp)
	fmt.Printf("  ntpTime: %d\n", ntpTime)
	// fmt.Printf("  ntpUtcTime: %s\n", ntpUtcTime)
	fmt.Printf("  clockRate: %d\n", clockRate)
	fmt.Printf("----------------------------\n")

	newSenderReport := SenderReport{
		RtpTimeNormalized: timestamp,
		UtcTime:           ntpUtcTime,
	}

	if _, exists := firstSrMap[ssrc]; !exists {
		firstSrMap[ssrc] = newSenderReport
	}

	firstSr := firstSrMap[ssrc]
	firstSrTime := TimestampToUtcTime(firstSr.RtpTimeNormalized, firstSr)
	currSrTime := TimestampToUtcTime(firstSr.RtpTimeNormalized, newSenderReport)

	duration := firstSrTime.Sub(currSrTime)
	deltaMs := duration.Milliseconds()
	firstSenderReportDelta.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(deltaMs))

	if oldSenderReport, exists := lastSrMap[ssrc]; exists {

		newDerivedTime := TimestampToUtcTime(timestamp, newSenderReport)
		oldDerivedTime := TimestampToUtcTime(timestamp, oldSenderReport)

		duration := newDerivedTime.Sub(oldDerivedTime)
		deltaMs := duration.Milliseconds()

		senderReportDelta.WithLabelValues(ssrc, direction, clockRateLabel).Set(float64(deltaMs))
	} else {
		senderReportDelta.WithLabelValues(ssrc, direction, clockRateLabel).Set(0.0)
	}

	lastSrMap[ssrc] = newSenderReport
}

func RtpNormalize100ns(rtpTime uint32, clockRate uint32) uint64 {
	norm := float64(rtpTime) / float64(clockRate) * 10_000_000
	return uint64(norm)
}

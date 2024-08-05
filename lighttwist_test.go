package prometheus

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/livekit/livekit-server/pkg/telemetry/prometheus"
	"github.com/livekit/mediatransportutil"
)

func TestTimestampToUtcTime(t *testing.T) {

	ts, _ := time.Parse(time.RFC3339, "2023-08-29T07:47:40.515Z")

	sr := prometheus.SenderReport{
		RtpTimeNormalized: 279811740000,
		UtcTime:           ts,
	}

	timestamp := uint64(279839100000)

	result := prometheus.TimestampToUtcTime(timestamp, sr)

	tsNew, _ := time.Parse(time.RFC3339, "2023-08-29T07:47:43.251Z")

	if result != tsNew {
		t.Errorf("TimestampToUtcTime() = %s; want %s", result, tsNew)
	}
}

func TestCompareNtpConversion(t *testing.T) {
	ntpTime := uint64(16884643893772862032)
	ntpMediaTransportUtil := mediatransportutil.NtpTime(ntpTime)
	ntpToTime := prometheus.NtpToTime(ntpTime)

	seconds := ntpTime >> 32
	fraction := ntpTime & 0xFFFFFFFF

	fmt.Printf("seconds: %d\n", seconds)
	fmt.Printf("fraction: %d\n", fraction)
	fmt.Printf("ntp: %s\n", ntpToTime)

	if !TimeEquals(ntpMediaTransportUtil.Time(), ntpToTime, time.Nanosecond) {
		t.Errorf("conversion doesn't match = %s; want %s", ntpMediaTransportUtil.Time(), ntpToTime)
	}
}

func TestRtpConversion(t *testing.T) {
	rtpTimeT0 := uint32(1513284410)
	rtpTimeT1 := uint32(1513554682)
	rtpTimeT2 := uint32(1513824732)

	clockRate := uint32(90000)

	timestampT0 := prometheus.RtpNormalize100ns(rtpTimeT0, clockRate)
	timestampT1 := prometheus.RtpNormalize100ns(rtpTimeT1, clockRate)
	timestampT2 := prometheus.RtpNormalize100ns(rtpTimeT2, clockRate)

	fmt.Printf("t0: %d\n", timestampT0)
	fmt.Printf("t1: %d\n", timestampT1)
	fmt.Printf("t2: %d\n", timestampT2)
}

func TestTimeAdd(t *testing.T) {
	baseTs, _ := time.Parse(time.RFC3339, "2023-08-29T07:47:43.251Z")

	delta100ns := 1

	converted1 := baseTs.Add(time.Duration(delta100ns*100) * time.Nanosecond)

	fmt.Printf("converted1: %s\n", converted1)

	converted2 := baseTs.Add(time.Duration(delta100ns) * (100 * time.Nanosecond))

	fmt.Printf("converted2: %s\n", converted2)
}

func Float64Equals(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TimeEquals(t1, t2 time.Time, tolerance time.Duration) bool {
	return absDuration(t1.Sub(t2)) <= tolerance
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

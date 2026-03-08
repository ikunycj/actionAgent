package observability

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Logger interface {
	Info(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}

type StdLogger struct {
	inner *log.Logger
}

func NewStdLogger() *StdLogger {
	return &StdLogger{inner: log.New(os.Stdout, "", 0)}
}

func (l *StdLogger) Info(msg string, fields map[string]any) {
	l.write("info", msg, fields)
}

func (l *StdLogger) Error(msg string, fields map[string]any) {
	l.write("error", msg, fields)
}

func (l *StdLogger) write(level, msg string, fields map[string]any) {
	m := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   msg,
	}
	for k, v := range fields {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	l.inner.Println(string(b))
}

type Metrics struct {
	taskSuccess uint64
	taskFail    uint64
	queueDepth  int64
	queueWaitMs uint64
	active      int64
	nodeOnline  int64
	approvalOK  uint64
	approvalTO  uint64

	mu     sync.RWMutex
	byName map[string]uint64
	gauges map[string]int64
}

func NewMetrics() *Metrics {
	return &Metrics{byName: map[string]uint64{}, gauges: map[string]int64{}}
}

func (m *Metrics) IncTaskSuccess() { atomic.AddUint64(&m.taskSuccess, 1) }
func (m *Metrics) IncTaskFail()    { atomic.AddUint64(&m.taskFail, 1) }
func (m *Metrics) AddQueueWait(ms uint64) {
	atomic.AddUint64(&m.queueWaitMs, ms)
}
func (m *Metrics) SetQueueDepth(v int64) { atomic.StoreInt64(&m.queueDepth, v) }
func (m *Metrics) SetActive(v int64)     { atomic.StoreInt64(&m.active, v) }
func (m *Metrics) SetNodeOnline(v int64) { atomic.StoreInt64(&m.nodeOnline, v) }
func (m *Metrics) IncApprovalOK()        { atomic.AddUint64(&m.approvalOK, 1) }
func (m *Metrics) IncApprovalTimeout()   { atomic.AddUint64(&m.approvalTO, 1) }

func (m *Metrics) Snapshot() map[string]any {
	out := map[string]any{
		"task_success":      atomic.LoadUint64(&m.taskSuccess),
		"task_fail":         atomic.LoadUint64(&m.taskFail),
		"queue_depth":       atomic.LoadInt64(&m.queueDepth),
		"queue_wait_ms":     atomic.LoadUint64(&m.queueWaitMs),
		"active_concurrent": atomic.LoadInt64(&m.active),
		"node_online":       atomic.LoadInt64(&m.nodeOnline),
		"approval_ok":       atomic.LoadUint64(&m.approvalOK),
		"approval_timeout":  atomic.LoadUint64(&m.approvalTO),
	}
	m.mu.RLock()
	for k, v := range m.byName {
		out[k] = v
	}
	for k, v := range m.gauges {
		out[k] = v
	}
	m.mu.RUnlock()
	return out
}

func (m *Metrics) Inc(name string) {
	m.mu.Lock()
	m.byName[name]++
	m.mu.Unlock()
}

func (m *Metrics) SetGauge(name string, v int64) {
	m.mu.Lock()
	m.gauges[name] = v
	m.mu.Unlock()
}

type AlertSeverity string

const (
	SeverityWarn  AlertSeverity = "warn"
	SeverityError AlertSeverity = "error"
)

type Alert struct {
	Code      string        `json:"code"`
	Severity  AlertSeverity `json:"severity"`
	Message   string        `json:"message"`
	Value     float64       `json:"value"`
	Threshold float64       `json:"threshold"`
}

type AlertThresholds struct {
	QueueDepthWarn      int64
	ApprovalTimeoutWarn uint64
	NodeOnlineMin       int64
	TaskFailureRateWarn float64
	MinSamples          int64
}

func DefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{
		QueueDepthWarn:      8,
		ApprovalTimeoutWarn: 3,
		NodeOnlineMin:       1,
		TaskFailureRateWarn: 0.30,
		MinSamples:          5,
	}
}

func EvaluateAlerts(snapshot map[string]any, th AlertThresholds) []Alert {
	alerts := []Alert{}
	queueDepth := asInt64(snapshot["queue_depth"])
	approvalTO := asUint64(snapshot["approval_timeout"])
	nodeOnline := asInt64(snapshot["node_online"])
	taskSuccess := asInt64(snapshot["task_success"])
	taskFail := asInt64(snapshot["task_fail"])

	if queueDepth >= th.QueueDepthWarn {
		alerts = append(alerts, Alert{
			Code:      "queue_depth_high",
			Severity:  SeverityWarn,
			Message:   "queue depth exceeded warning threshold",
			Value:     float64(queueDepth),
			Threshold: float64(th.QueueDepthWarn),
		})
	}
	if int64(approvalTO) >= int64(th.ApprovalTimeoutWarn) {
		alerts = append(alerts, Alert{
			Code:      "approval_timeout_high",
			Severity:  SeverityWarn,
			Message:   "approval timeout count exceeded warning threshold",
			Value:     float64(approvalTO),
			Threshold: float64(th.ApprovalTimeoutWarn),
		})
	}
	if nodeOnline < th.NodeOnlineMin {
		alerts = append(alerts, Alert{
			Code:      "node_online_low",
			Severity:  SeverityError,
			Message:   "online nodes below minimum threshold",
			Value:     float64(nodeOnline),
			Threshold: float64(th.NodeOnlineMin),
		})
	}

	total := taskSuccess + taskFail
	if total >= th.MinSamples && total > 0 {
		failRate := float64(taskFail) / float64(total)
		if failRate >= th.TaskFailureRateWarn {
			alerts = append(alerts, Alert{
				Code:      "task_failure_rate_high",
				Severity:  SeverityWarn,
				Message:   "task failure rate exceeded warning threshold",
				Value:     roundTo(failRate, 4),
				Threshold: th.TaskFailureRateWarn,
			})
		}
	}
	return alerts
}

func roundTo(v float64, digits int) float64 {
	if digits < 0 {
		return v
	}
	base := math.Pow(10, float64(digits))
	return math.Round(v*base) / base
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case uint64:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func asUint64(v any) uint64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		if x < 0 {
			return 0
		}
		return uint64(x)
	case int64:
		if x < 0 {
			return 0
		}
		return uint64(x)
	case uint64:
		return x
	case float64:
		if x < 0 {
			return 0
		}
		return uint64(x)
	case json.Number:
		n, _ := x.Int64()
		if n < 0 {
			return 0
		}
		return uint64(n)
	case string:
		n, err := strconv.ParseUint(x, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

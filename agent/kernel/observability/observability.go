package observability

import (
	"encoding/json"
	"log"
	"os"
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

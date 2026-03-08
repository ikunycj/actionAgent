package observability

import "testing"

func TestEvaluateAlerts(t *testing.T) {
	alerts := EvaluateAlerts(map[string]any{
		"queue_depth":      int64(12),
		"approval_timeout": uint64(4),
		"node_online":      int64(0),
		"task_success":     uint64(6),
		"task_fail":        uint64(4),
	}, DefaultAlertThresholds())
	if len(alerts) < 3 {
		t.Fatalf("expected multiple alerts, got %d", len(alerts))
	}
}

func TestEvaluateAlertsNoSignal(t *testing.T) {
	alerts := EvaluateAlerts(map[string]any{
		"queue_depth":      int64(1),
		"approval_timeout": uint64(0),
		"node_online":      int64(2),
		"task_success":     uint64(10),
		"task_fail":        uint64(0),
	}, DefaultAlertThresholds())
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts, got %d", len(alerts))
	}
}

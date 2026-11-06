package server

import (
	"strings"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

func TestMergeScheduleKeepsStoreOrder(t *testing.T) {
	a := []store.ScheduleItem{{ID: "u1", DueDate: "2026-10-22"}, {ID: "u2", DueDate: "2026-10-23", DueTime: "09:00"}}
	b := []store.ScheduleItem{{ID: "lms:demo:1", DueDate: "2026-10-23", DueTime: "08:00"}, {ID: "lms:demo:2", DueDate: "2026-10-21", DueTime: "23:59"}}
	var ids []string
	for _, it := range mergeSchedule(a, b) {
		ids = append(ids, it.ID)
	}
	if got := strings.Join(ids, ","); got != "lms:demo:2,u1,lms:demo:1,u2" {
		t.Fatal(got)
	}
}

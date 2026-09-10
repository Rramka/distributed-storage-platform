package repair

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Rramka/distributed-storage-platform/internal/events"
)

func TestWorkerHoldsNoSigningSeed(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(Worker{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		name := strings.ToLower(f.Name)
		typ := f.Type.String()
		if strings.Contains(name, "seed") || strings.Contains(typ, "tickets.Signer") {
			t.Fatalf("repair worker must not hold a signing seed: %s %s", f.Name, typ)
		}
		if strings.Contains(strings.ToLower(typ), "encryption") {
			t.Fatalf("repair worker must not hold encryption_meta: %s %s", f.Name, typ)
		}
	}
}

func TestPriorityMatchesSpec(t *testing.T) {
	t.Parallel()
	if events.PrioritySubject(13) != "" {
		t.Fatal("≥13 must skip")
	}
	if events.PrioritySubject(12) != events.SubjRepairNormal {
		t.Fatal("12 normal")
	}
	if events.PrioritySubject(11) != events.SubjRepairHigh {
		t.Fatal("11 high")
	}
	if events.PrioritySubject(10) != events.SubjRepairCritical {
		t.Fatal("≤10 critical")
	}
}

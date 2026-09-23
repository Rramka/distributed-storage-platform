package ledger

import (
	"testing"
	"testing/quick"

	"github.com/google/uuid"
)

func TestSumZeroRejectsUnbalanced(t *testing.T) {
	t.Parallel()
	fn := func(a, b int64) bool {
		if a+b == 0 && (a != 0 || b != 0) {
			return SumZero([]Entry{
				{AccountID: uuid.New(), Amount: a, Type: TypeStorageCharge},
				{AccountID: uuid.New(), Amount: b, Type: TypeStorageCharge},
			}) == nil
		}
		err := SumZero([]Entry{
			{AccountID: uuid.New(), Amount: a, Type: TypeStorageCharge},
			{AccountID: uuid.New(), Amount: b + 1, Type: TypeStorageCharge},
		})
		return err != nil
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 64}); err != nil {
		t.Fatal(err)
	}
}

func TestPostTxnPropertyUnbalancedRejected(t *testing.T) {
	t.Parallel()
	l := &Ledger{}
	fn := func(n int16) bool {
		amt := int64(n)
		if amt == 0 {
			amt = 1
		}
		err := l.PostTxn(t.Context(), uuid.New(), []Entry{
			{AccountID: uuid.New(), Amount: amt, Type: TypeStorageCharge},
		})
		return err != nil
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 64}); err != nil {
		t.Fatal(err)
	}
}

func TestStorageChargeInteger(t *testing.T) {
	t.Parallel()
	r := DefaultRates()
	if StorageChargeµCRD(0, r) != 0 {
		t.Fatal("zero bytes")
	}
	got := StorageChargeµCRD(BytesPerGB, r)
	want := r.CustomerStoragePerGBMonth / HoursPerMonth
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestReliabilityBPDegradedLess(t *testing.T) {
	t.Parallel()
	clean := ReliabilityBP(1, 1, 1)
	bad := ReliabilityBP(0.5, 1, 1)
	if bad >= clean {
		t.Fatalf("degraded %d clean %d", bad, clean)
	}
	if StorageEarningµCRD(BytesPerGB, DefaultRates(), bad) >= StorageEarningµCRD(BytesPerGB, DefaultRates(), clean) {
		t.Fatal("earning should drop with R")
	}
}

func TestSumZeroEmpty(t *testing.T) {
	t.Parallel()
	if SumZero(nil) == nil {
		t.Fatal("empty")
	}
}

package travel

import (
	"context"
	"testing"
)

func TestDriveBuysFuelATankAtATime(t *testing.T) {
	Offline = true
	f := NewFuel("")
	// 606 miles at 28 mpg with a 14 gal tank: range 392, usable 333 per fill.
	d := f.Drive(context.Background(), 606, 621, 28, 14)
	if d.TankGallons != 14 || d.RangeMiles != 392 {
		t.Errorf("tank/range: %+v", d)
	}
	if len(d.Refuels) != 1 || d.Refuels[0].AtMile != 333.2 {
		t.Fatalf("one refuel around mile 333: %+v", d.Refuels)
	}
	total := d.StartTankUSD
	for _, r := range d.Refuels {
		total += r.CostUSD
	}
	if diff := total - d.FuelCostUSD; diff > 0.05 || diff < -0.05 {
		t.Errorf("tank split %.2f must add up to the one-way cost %.2f", total, d.FuelCostUSD)
	}
	// A short hop needs no refuel and the first tank is the whole cost.
	short := f.Drive(context.Background(), 120, 130, 28, 14)
	if len(short.Refuels) != 0 || short.StartTankUSD != short.FuelCostUSD {
		t.Errorf("short drive: %+v", short)
	}
	// Unknown tank size falls back to the default.
	if f.Drive(context.Background(), 10, 10, 28, 0).TankGallons != DefaultTankGallons {
		t.Error("default tank")
	}
}

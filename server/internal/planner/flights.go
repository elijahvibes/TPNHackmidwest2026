package planner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"trailmix/internal/travel"
)

func FlightContext(plan *Plan, index int) (travel.FlightSearch, error) {
	if index < 0 || index >= len(plan.Flights) || plan.Travelers < 1 {
		return travel.FlightSearch{}, fmt.Errorf("invalid flight selection; re-plan older trips to enable flight selection")
	}
	f := plan.Flights[index]
	return travel.FlightSearch{
		Origin: f.Origin, Destination: f.Destination, Depart: f.Depart, Return: f.Return,
		Travelers: plan.Travelers, DepartureToken: f.DepartureToken,
	}, nil
}

func (p *Planner) SelectFlight(ctx context.Context, plan *Plan, index int, returnToken string) error {
	if plan.Nights < 1 || len(plan.Days) != plan.Nights+1 {
		return fmt.Errorf("invalid plan; re-plan before selecting a flight")
	}
	search, err := FlightContext(plan, index)
	if err != nil {
		return err
	}
	start, err := time.Parse("2006-01-02", search.Depart)
	if err != nil {
		return fmt.Errorf("invalid flight departure date")
	}
	f := plan.Flights[index]
	var ret *travel.FlightQuote
	if returnToken != "" {
		returns, err := p.SerpAPI.ReturnFlights(ctx, search)
		if err != nil {
			return err
		}
		for i := range returns {
			if returns[i].BookingToken == returnToken {
				ret = &returns[i]
				break
			}
		}
		if ret == nil {
			return fmt.Errorf("return flight is no longer available; refresh return options")
		}
	}
	price := f.PriceUSD
	duration := float64(f.DurationMin+150) * 2 / 60
	if ret != nil {
		price = ret.PriceUSD
		duration = float64(f.DurationMin+ret.DurationMin+300) / 60
	}
	if plan.OriginAirport != nil && plan.DestAirport != nil {
		duration += (plan.OriginAirport.DistanceMiles + plan.DestAirport.DistanceMiles) * 2 / 45
	}
	found := false
	for i := range plan.Options {
		o := &plan.Options[i]
		if o.Mode != "fly" {
			continue
		}
		found = true
		o.TransportUSD = round2(price * float64(plan.Travelers))
		o.TravelHours = round1(duration)
		o.TotalUSD = round2(o.TransportUSD + o.LodgingUSD + o.ActivitiesUSD + o.LocalUSD)
		o.ValueScore = valueScore(*o, plan.Nights)
		o.Why = fmt.Sprintf("%s round trip from $%.2f/person, %d outbound stop(s)", f.Carrier, price, f.Stops)
		factors := []string{
			fmt.Sprintf("$%.0f per night all-in (estimates included)", o.TotalUSD/float64(plan.Nights)),
			fmt.Sprintf("%.1f h door to door round trip incl. airport time", o.TravelHours),
			fmt.Sprintf("$%.0f/person × %d = $%.0f airfare", price, plan.Travelers, o.TransportUSD),
		}
		if len(o.Factors) > 3 {
			factors = append(factors, o.Factors[3:]...)
		}
		o.Factors = factors
		plan.TotalUSD = o.TotalUSD
	}
	if !found {
		return fmt.Errorf("this plan has no flight option; re-plan the trip")
	}
	plan.SelectedFlight, plan.SelectedReturn = index, ret
	plan.FlightSelected = true
	plan.RecommendedMode = "fly"
	plan.Stops = make([]travel.Place, 0)
	plan.Sources["places"] = placesSource(plan.Activities, plan.Food)
	plan.UnderBudget = plan.BudgetUSD <= 0 || plan.TotalUSD <= plan.BudgetUSD
	req := Request{Origin: plan.OriginName, Destination: plan.DestinationName,
		Travelers: plan.Travelers, BudgetUSD: plan.BudgetUSD,
		LocalTransport: plan.LocalTransport, ActivityRadiusMiles: plan.ActivityRadiusMiles}
	plan.Summary, plan.Days = fallbackItinerary(req, plan, start)
	plan.Summary = strings.Replace(plan.Summary, "Best value is to fly:", "You chose to fly:", 1)
	plan.AIGenerated = false
	delete(plan.Sources, "ai")
	return nil
}

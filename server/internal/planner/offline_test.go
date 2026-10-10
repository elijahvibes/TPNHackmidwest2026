package planner

import "trailmix/internal/travel"

// Unit tests never call the keyless public providers.
func init() { travel.Offline = true }

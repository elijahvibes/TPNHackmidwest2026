package travel

// Offline disables the keyless public providers (OSRM routing, the EIA gas
// feed, the FBI crime API) so unit tests stay deterministic and fast; they
// fall back to the built-in estimates. Test packages set it in an init().
var Offline bool

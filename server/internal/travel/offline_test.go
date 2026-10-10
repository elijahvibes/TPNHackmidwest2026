package travel

// Unit tests never call the keyless public providers.
func init() { Offline = true }

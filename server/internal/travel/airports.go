package travel

import "strings"

// Airport is a commercial airport the flight search can use. Any city gets
// flights priced from its nearest one, with the drive to it shown in the plan.
type Airport struct {
	Code string `json:"code"` // IATA
	Name string `json:"name"`
	City string `json:"city"`
	LatLng
	DistanceMiles float64 `json:"distance_miles"` // from the place the user asked for
}

// Major US airports with scheduled passenger service. Hubs first in each
// region so ties resolve toward the airport with more routes.
var airports = []Airport{
	{Code: "MCI", Name: "Kansas City Intl", City: "Kansas City", LatLng: LatLng{39.2976, -94.7139}},
	{Code: "STL", Name: "St. Louis Lambert", City: "St. Louis", LatLng: LatLng{38.7500, -90.3700}},
	{Code: "SGF", Name: "Springfield-Branson", City: "Springfield", LatLng: LatLng{37.2457, -93.3886}},
	{Code: "ICT", Name: "Wichita Eisenhower", City: "Wichita", LatLng: LatLng{37.6499, -97.4331}},
	{Code: "OMA", Name: "Eppley Airfield", City: "Omaha", LatLng: LatLng{41.3032, -95.8941}},
	{Code: "LNK", Name: "Lincoln Airport", City: "Lincoln", LatLng: LatLng{40.8510, -96.7592}},
	{Code: "DSM", Name: "Des Moines Intl", City: "Des Moines", LatLng: LatLng{41.5340, -93.6631}},
	{Code: "CID", Name: "Eastern Iowa", City: "Cedar Rapids", LatLng: LatLng{41.8847, -91.7108}},
	{Code: "MLI", Name: "Quad City Intl", City: "Moline", LatLng: LatLng{41.4485, -90.5075}},
	{Code: "OKC", Name: "Will Rogers World", City: "Oklahoma City", LatLng: LatLng{35.3931, -97.6007}},
	{Code: "TUL", Name: "Tulsa Intl", City: "Tulsa", LatLng: LatLng{36.1984, -95.8881}},
	{Code: "XNA", Name: "Northwest Arkansas", City: "Bentonville", LatLng: LatLng{36.2819, -94.3068}},
	{Code: "LIT", Name: "Clinton National", City: "Little Rock", LatLng: LatLng{34.7294, -92.2243}},
	{Code: "ORD", Name: "Chicago O'Hare", City: "Chicago", LatLng: LatLng{41.9742, -87.9073}},
	{Code: "MDW", Name: "Chicago Midway", City: "Chicago", LatLng: LatLng{41.7868, -87.7522}},
	{Code: "PIA", Name: "Peoria Intl", City: "Peoria", LatLng: LatLng{40.6642, -89.6933}},
	{Code: "SPI", Name: "Abraham Lincoln Capital", City: "Springfield", LatLng: LatLng{39.8441, -89.6779}},
	{Code: "MKE", Name: "Milwaukee Mitchell", City: "Milwaukee", LatLng: LatLng{42.9472, -87.8966}},
	{Code: "MSN", Name: "Dane County", City: "Madison", LatLng: LatLng{43.1399, -89.3375}},
	{Code: "GRB", Name: "Green Bay Austin Straubel", City: "Green Bay", LatLng: LatLng{44.4851, -88.1296}},
	{Code: "MSP", Name: "Minneapolis-St Paul", City: "Minneapolis", LatLng: LatLng{44.8848, -93.2223}},
	{Code: "DLH", Name: "Duluth Intl", City: "Duluth", LatLng: LatLng{46.8421, -92.1936}},
	{Code: "FSD", Name: "Sioux Falls Regional", City: "Sioux Falls", LatLng: LatLng{43.5820, -96.7419}},
	{Code: "RAP", Name: "Rapid City Regional", City: "Rapid City", LatLng: LatLng{44.0453, -103.0574}},
	{Code: "FAR", Name: "Hector Intl", City: "Fargo", LatLng: LatLng{46.9207, -96.8158}},
	{Code: "BIS", Name: "Bismarck Airport", City: "Bismarck", LatLng: LatLng{46.7727, -100.7467}},
	{Code: "IND", Name: "Indianapolis Intl", City: "Indianapolis", LatLng: LatLng{39.7173, -86.2944}},
	{Code: "FWA", Name: "Fort Wayne Intl", City: "Fort Wayne", LatLng: LatLng{40.9785, -85.1951}},
	{Code: "SBN", Name: "South Bend Intl", City: "South Bend", LatLng: LatLng{41.7087, -86.3173}},
	{Code: "DTW", Name: "Detroit Metro", City: "Detroit", LatLng: LatLng{42.2162, -83.3554}},
	{Code: "GRR", Name: "Gerald R. Ford Intl", City: "Grand Rapids", LatLng: LatLng{42.8808, -85.5228}},
	{Code: "TVC", Name: "Cherry Capital", City: "Traverse City", LatLng: LatLng{44.7414, -85.5822}},
	{Code: "CMH", Name: "John Glenn Columbus", City: "Columbus", LatLng: LatLng{39.9980, -82.8919}},
	{Code: "CVG", Name: "Cincinnati/N. Kentucky", City: "Cincinnati", LatLng: LatLng{39.0489, -84.6678}},
	{Code: "CLE", Name: "Cleveland Hopkins", City: "Cleveland", LatLng: LatLng{41.4117, -81.8498}},
	{Code: "DAY", Name: "Dayton Intl", City: "Dayton", LatLng: LatLng{39.9024, -84.2194}},
	{Code: "SDF", Name: "Louisville Muhammad Ali", City: "Louisville", LatLng: LatLng{38.1744, -85.7360}},
	{Code: "LEX", Name: "Blue Grass", City: "Lexington", LatLng: LatLng{38.0365, -84.6059}},
	{Code: "BNA", Name: "Nashville Intl", City: "Nashville", LatLng: LatLng{36.1263, -86.6774}},
	{Code: "MEM", Name: "Memphis Intl", City: "Memphis", LatLng: LatLng{35.0424, -89.9767}},
	{Code: "TYS", Name: "McGhee Tyson", City: "Knoxville", LatLng: LatLng{35.8110, -83.9940}},
	{Code: "CHA", Name: "Chattanooga Metro", City: "Chattanooga", LatLng: LatLng{35.0353, -85.2038}},
	{Code: "DEN", Name: "Denver Intl", City: "Denver", LatLng: LatLng{39.8561, -104.6737}},
	{Code: "COS", Name: "Colorado Springs", City: "Colorado Springs", LatLng: LatLng{38.8058, -104.7008}},
	{Code: "ASE", Name: "Aspen/Pitkin County", City: "Aspen", LatLng: LatLng{39.2232, -106.8688}},
	{Code: "EGE", Name: "Eagle County", City: "Vail", LatLng: LatLng{39.6426, -106.9176}},
	{Code: "HDN", Name: "Yampa Valley", City: "Steamboat Springs", LatLng: LatLng{40.4812, -107.2177}},
	{Code: "GJT", Name: "Grand Junction Regional", City: "Grand Junction", LatLng: LatLng{39.1224, -108.5267}},
	{Code: "DRO", Name: "Durango-La Plata", City: "Durango", LatLng: LatLng{37.1515, -107.7538}},
	{Code: "JAC", Name: "Jackson Hole", City: "Jackson", LatLng: LatLng{43.6073, -110.7377}},
	{Code: "BZN", Name: "Bozeman Yellowstone", City: "Bozeman", LatLng: LatLng{45.7775, -111.1530}},
	{Code: "MSO", Name: "Missoula Montana", City: "Missoula", LatLng: LatLng{46.9163, -114.0906}},
	{Code: "BIL", Name: "Billings Logan", City: "Billings", LatLng: LatLng{45.8077, -108.5429}},
	{Code: "FCA", Name: "Glacier Park Intl", City: "Kalispell", LatLng: LatLng{48.3105, -114.2560}},
	{Code: "COD", Name: "Yellowstone Regional", City: "Cody", LatLng: LatLng{44.5202, -109.0238}},
	{Code: "CYS", Name: "Cheyenne Regional", City: "Cheyenne", LatLng: LatLng{41.1557, -104.8119}},
	{Code: "SLC", Name: "Salt Lake City Intl", City: "Salt Lake City", LatLng: LatLng{40.7899, -111.9791}},
	{Code: "SGU", Name: "St. George Regional", City: "St. George", LatLng: LatLng{37.0364, -113.5103}},
	{Code: "CNY", Name: "Canyonlands Regional", City: "Moab", LatLng: LatLng{38.7550, -109.7548}},
	{Code: "BOI", Name: "Boise Airport", City: "Boise", LatLng: LatLng{43.5644, -116.2228}},
	{Code: "ABQ", Name: "Albuquerque Sunport", City: "Albuquerque", LatLng: LatLng{35.0402, -106.6090}},
	{Code: "SAF", Name: "Santa Fe Regional", City: "Santa Fe", LatLng: LatLng{35.6171, -106.0881}},
	{Code: "ELP", Name: "El Paso Intl", City: "El Paso", LatLng: LatLng{31.8072, -106.3776}},
	{Code: "PHX", Name: "Phoenix Sky Harbor", City: "Phoenix", LatLng: LatLng{33.4373, -112.0078}},
	{Code: "TUS", Name: "Tucson Intl", City: "Tucson", LatLng: LatLng{32.1161, -110.9410}},
	{Code: "FLG", Name: "Flagstaff Pulliam", City: "Flagstaff", LatLng: LatLng{35.1385, -111.6709}},
	{Code: "LAS", Name: "Harry Reid Intl", City: "Las Vegas", LatLng: LatLng{36.0840, -115.1537}},
	{Code: "RNO", Name: "Reno-Tahoe Intl", City: "Reno", LatLng: LatLng{39.4991, -119.7681}},
	{Code: "DFW", Name: "Dallas/Fort Worth Intl", City: "Dallas", LatLng: LatLng{32.8998, -97.0403}},
	{Code: "DAL", Name: "Dallas Love Field", City: "Dallas", LatLng: LatLng{32.8471, -96.8518}},
	{Code: "AUS", Name: "Austin-Bergstrom", City: "Austin", LatLng: LatLng{30.1975, -97.6664}},
	{Code: "IAH", Name: "Houston Bush Intercontinental", City: "Houston", LatLng: LatLng{29.9902, -95.3368}},
	{Code: "HOU", Name: "Houston Hobby", City: "Houston", LatLng: LatLng{29.6454, -95.2789}},
	{Code: "SAT", Name: "San Antonio Intl", City: "San Antonio", LatLng: LatLng{29.5337, -98.4698}},
	{Code: "CRP", Name: "Corpus Christi Intl", City: "Corpus Christi", LatLng: LatLng{27.7704, -97.5012}},
	{Code: "AMA", Name: "Rick Husband Amarillo", City: "Amarillo", LatLng: LatLng{35.2194, -101.7059}},
	{Code: "LBB", Name: "Lubbock Preston Smith", City: "Lubbock", LatLng: LatLng{33.6636, -101.8228}},
	{Code: "MAF", Name: "Midland Intl", City: "Midland", LatLng: LatLng{31.9425, -102.2019}},
	{Code: "MSY", Name: "Louis Armstrong New Orleans", City: "New Orleans", LatLng: LatLng{29.9934, -90.2580}},
	{Code: "BTR", Name: "Baton Rouge Metro", City: "Baton Rouge", LatLng: LatLng{30.5332, -91.1496}},
	{Code: "SHV", Name: "Shreveport Regional", City: "Shreveport", LatLng: LatLng{32.4466, -93.8256}},
	{Code: "JAN", Name: "Jackson-Medgar Evers", City: "Jackson", LatLng: LatLng{32.3112, -90.0759}},
	{Code: "GPT", Name: "Gulfport-Biloxi", City: "Gulfport", LatLng: LatLng{30.4073, -89.0701}},
	{Code: "MOB", Name: "Mobile Regional", City: "Mobile", LatLng: LatLng{30.6914, -88.2428}},
	{Code: "BHM", Name: "Birmingham-Shuttlesworth", City: "Birmingham", LatLng: LatLng{33.5629, -86.7535}},
	{Code: "HSV", Name: "Huntsville Intl", City: "Huntsville", LatLng: LatLng{34.6372, -86.7751}},
	{Code: "ATL", Name: "Hartsfield-Jackson Atlanta", City: "Atlanta", LatLng: LatLng{33.6407, -84.4277}},
	{Code: "SAV", Name: "Savannah/Hilton Head", City: "Savannah", LatLng: LatLng{32.1276, -81.2021}},
	{Code: "MIA", Name: "Miami Intl", City: "Miami", LatLng: LatLng{25.7959, -80.2870}},
	{Code: "FLL", Name: "Fort Lauderdale-Hollywood", City: "Fort Lauderdale", LatLng: LatLng{26.0742, -80.1506}},
	{Code: "PBI", Name: "Palm Beach Intl", City: "West Palm Beach", LatLng: LatLng{26.6832, -80.0956}},
	{Code: "MCO", Name: "Orlando Intl", City: "Orlando", LatLng: LatLng{28.4312, -81.3081}},
	{Code: "TPA", Name: "Tampa Intl", City: "Tampa", LatLng: LatLng{27.9772, -82.5311}},
	{Code: "RSW", Name: "Southwest Florida Intl", City: "Fort Myers", LatLng: LatLng{26.5362, -81.7552}},
	{Code: "SRQ", Name: "Sarasota-Bradenton", City: "Sarasota", LatLng: LatLng{27.3954, -82.5544}},
	{Code: "JAX", Name: "Jacksonville Intl", City: "Jacksonville", LatLng: LatLng{30.4941, -81.6879}},
	{Code: "PNS", Name: "Pensacola Intl", City: "Pensacola", LatLng: LatLng{30.4734, -87.1866}},
	{Code: "VPS", Name: "Destin-Fort Walton Beach", City: "Destin", LatLng: LatLng{30.4832, -86.5254}},
	{Code: "ECP", Name: "Northwest Florida Beaches", City: "Panama City Beach", LatLng: LatLng{30.3571, -85.7954}},
	{Code: "TLH", Name: "Tallahassee Intl", City: "Tallahassee", LatLng: LatLng{30.3965, -84.3503}},
	{Code: "EYW", Name: "Key West Intl", City: "Key West", LatLng: LatLng{24.5561, -81.7596}},
	{Code: "CLT", Name: "Charlotte Douglas", City: "Charlotte", LatLng: LatLng{35.2144, -80.9473}},
	{Code: "RDU", Name: "Raleigh-Durham", City: "Raleigh", LatLng: LatLng{35.8801, -78.7880}},
	{Code: "AVL", Name: "Asheville Regional", City: "Asheville", LatLng: LatLng{35.4362, -82.5418}},
	{Code: "GSO", Name: "Piedmont Triad", City: "Greensboro", LatLng: LatLng{36.0978, -79.9373}},
	{Code: "ILM", Name: "Wilmington Intl", City: "Wilmington", LatLng: LatLng{34.2706, -77.9026}},
	{Code: "CHS", Name: "Charleston Intl", City: "Charleston", LatLng: LatLng{32.8986, -80.0405}},
	{Code: "MYR", Name: "Myrtle Beach Intl", City: "Myrtle Beach", LatLng: LatLng{33.6797, -78.9283}},
	{Code: "GSP", Name: "Greenville-Spartanburg", City: "Greenville", LatLng: LatLng{34.8957, -82.2189}},
	{Code: "CAE", Name: "Columbia Metropolitan", City: "Columbia", LatLng: LatLng{33.9388, -81.1195}},
	{Code: "DCA", Name: "Reagan National", City: "Washington", LatLng: LatLng{38.8512, -77.0402}},
	{Code: "IAD", Name: "Washington Dulles", City: "Washington", LatLng: LatLng{38.9531, -77.4565}},
	{Code: "BWI", Name: "Baltimore/Washington", City: "Baltimore", LatLng: LatLng{39.1774, -76.6684}},
	{Code: "RIC", Name: "Richmond Intl", City: "Richmond", LatLng: LatLng{37.5052, -77.3197}},
	{Code: "ORF", Name: "Norfolk Intl", City: "Norfolk", LatLng: LatLng{36.8946, -76.2012}},
	{Code: "ROA", Name: "Roanoke-Blacksburg", City: "Roanoke", LatLng: LatLng{37.3255, -79.9754}},
	{Code: "PHL", Name: "Philadelphia Intl", City: "Philadelphia", LatLng: LatLng{39.8744, -75.2424}},
	{Code: "PIT", Name: "Pittsburgh Intl", City: "Pittsburgh", LatLng: LatLng{40.4915, -80.2329}},
	{Code: "MDT", Name: "Harrisburg Intl", City: "Harrisburg", LatLng: LatLng{40.1935, -76.7634}},
	{Code: "ERI", Name: "Erie Intl", City: "Erie", LatLng: LatLng{42.0831, -80.1739}},
	{Code: "EWR", Name: "Newark Liberty", City: "Newark", LatLng: LatLng{40.6895, -74.1745}},
	{Code: "JFK", Name: "New York JFK", City: "New York", LatLng: LatLng{40.6413, -73.7781}},
	{Code: "LGA", Name: "LaGuardia", City: "New York", LatLng: LatLng{40.7769, -73.8740}},
	{Code: "ACY", Name: "Atlantic City Intl", City: "Atlantic City", LatLng: LatLng{39.4576, -74.5772}},
	{Code: "BUF", Name: "Buffalo Niagara", City: "Buffalo", LatLng: LatLng{42.9405, -78.7322}},
	{Code: "ROC", Name: "Greater Rochester", City: "Rochester", LatLng: LatLng{43.1189, -77.6724}},
	{Code: "SYR", Name: "Syracuse Hancock", City: "Syracuse", LatLng: LatLng{43.1112, -76.1063}},
	{Code: "ALB", Name: "Albany Intl", City: "Albany", LatLng: LatLng{42.7483, -73.8017}},
	{Code: "BOS", Name: "Boston Logan", City: "Boston", LatLng: LatLng{42.3656, -71.0096}},
	{Code: "PVD", Name: "Rhode Island T.F. Green", City: "Providence", LatLng: LatLng{41.7240, -71.4282}},
	{Code: "BDL", Name: "Bradley Intl", City: "Hartford", LatLng: LatLng{41.9389, -72.6832}},
	{Code: "MHT", Name: "Manchester-Boston", City: "Manchester", LatLng: LatLng{42.9326, -71.4357}},
	{Code: "PWM", Name: "Portland Intl Jetport", City: "Portland", LatLng: LatLng{43.6462, -70.3093}},
	{Code: "BGR", Name: "Bangor Intl", City: "Bangor", LatLng: LatLng{44.8074, -68.8281}},
	{Code: "BTV", Name: "Burlington Intl", City: "Burlington", LatLng: LatLng{44.4719, -73.1533}},
	{Code: "LAX", Name: "Los Angeles Intl", City: "Los Angeles", LatLng: LatLng{33.9416, -118.4085}},
	{Code: "SNA", Name: "John Wayne Orange County", City: "Santa Ana", LatLng: LatLng{33.6762, -117.8675}},
	{Code: "BUR", Name: "Hollywood Burbank", City: "Burbank", LatLng: LatLng{34.2007, -118.3587}},
	{Code: "ONT", Name: "Ontario Intl", City: "Ontario", LatLng: LatLng{34.0560, -117.6012}},
	{Code: "SAN", Name: "San Diego Intl", City: "San Diego", LatLng: LatLng{32.7338, -117.1933}},
	{Code: "PSP", Name: "Palm Springs Intl", City: "Palm Springs", LatLng: LatLng{33.8297, -116.5067}},
	{Code: "SBA", Name: "Santa Barbara", City: "Santa Barbara", LatLng: LatLng{34.4262, -119.8415}},
	{Code: "SFO", Name: "San Francisco Intl", City: "San Francisco", LatLng: LatLng{37.6213, -122.3790}},
	{Code: "OAK", Name: "Oakland Intl", City: "Oakland", LatLng: LatLng{37.7126, -122.2197}},
	{Code: "SJC", Name: "San Jose Mineta", City: "San Jose", LatLng: LatLng{37.3639, -121.9289}},
	{Code: "SMF", Name: "Sacramento Intl", City: "Sacramento", LatLng: LatLng{38.6954, -121.5908}},
	{Code: "FAT", Name: "Fresno Yosemite", City: "Fresno", LatLng: LatLng{36.7762, -119.7181}},
	{Code: "MRY", Name: "Monterey Regional", City: "Monterey", LatLng: LatLng{36.5870, -121.8429}},
	{Code: "PDX", Name: "Portland Intl", City: "Portland", LatLng: LatLng{45.5898, -122.5951}},
	{Code: "EUG", Name: "Eugene Airport", City: "Eugene", LatLng: LatLng{44.1246, -123.2119}},
	{Code: "RDM", Name: "Redmond Municipal", City: "Bend", LatLng: LatLng{44.2541, -121.1500}},
	{Code: "MFR", Name: "Rogue Valley", City: "Medford", LatLng: LatLng{42.3742, -122.8735}},
	{Code: "SEA", Name: "Seattle-Tacoma Intl", City: "Seattle", LatLng: LatLng{47.4502, -122.3088}},
	{Code: "GEG", Name: "Spokane Intl", City: "Spokane", LatLng: LatLng{47.6199, -117.5338}},
	{Code: "HNL", Name: "Daniel K. Inouye", City: "Honolulu", LatLng: LatLng{21.3187, -157.9225}},
	{Code: "OGG", Name: "Kahului", City: "Maui", LatLng: LatLng{20.8986, -156.4305}},
	{Code: "ANC", Name: "Ted Stevens Anchorage", City: "Anchorage", LatLng: LatLng{61.1743, -149.9983}},
}

// NearestAirport picks the closest listed airport to a point. Nothing within
// 400 miles means flying is not a realistic option (Alaska bush, open ocean…)
// and ok is false.
func NearestAirport(at LatLng) (Airport, bool) {
	best, bestD := Airport{}, 1e9
	for _, a := range airports {
		if d := haversineMiles(at, a.LatLng); d < bestD {
			best, bestD = a, d
		}
	}
	if bestD > 400 {
		return Airport{}, false
	}
	best.DistanceMiles = round1(bestD)
	return best, true
}

// AirportByCode is a convenience for tests and seed data.
func AirportByCode(code string) (Airport, bool) {
	for _, a := range airports {
		if strings.EqualFold(a.Code, code) {
			return a, true
		}
	}
	return Airport{}, false
}

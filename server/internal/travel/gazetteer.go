package travel

import (
	"sort"
	"strings"
)

// Placemark is a resolved place: what the user typed turned into a point plus
// a short label ("Lawrence, KS") the UI can show back.
type Placemark struct {
	Label   string `json:"label"`   // "Lawrence, KS" / "Banff, Alberta, Canada"
	Name    string `json:"name"`    // "Lawrence"
	Region  string `json:"region"`  // "KS" (state/province code when known)
	Country string `json:"country"` // "US"
	LatLng
	Source string `json:"source"` // gazetteer | photon | nominatim | mapbox | gps
	// Coarse place type from the geocoder (city | town | village | park | island | locality).
	// The UI refines it from the name to pick an illustrative photo.
	Kind string `json:"kind,omitempty"`
}

type gazEntry struct {
	name  string
	state string // 2-letter
	ll    LatLng
}

// Built-in gazetteer: cities, towns and parks people in the Midwest actually
// drive to. It makes the demo work with zero keys and no network; anything
// else goes to a live geocoder (see geo.go).
var gazetteer = []gazEntry{
	// Kansas / Missouri
	{"Kansas City", "MO", LatLng{39.0997, -94.5786}}, {"Kansas City", "KS", LatLng{39.1142, -94.6275}},
	{"Overland Park", "KS", LatLng{38.9822, -94.6708}}, {"Olathe", "KS", LatLng{38.8814, -94.8191}},
	{"Lawrence", "KS", LatLng{38.9717, -95.2353}}, {"Topeka", "KS", LatLng{39.0473, -95.6752}},
	{"Manhattan", "KS", LatLng{39.1836, -96.5717}}, {"Wichita", "KS", LatLng{37.6872, -97.3301}},
	{"Salina", "KS", LatLng{38.8403, -97.6114}}, {"Hays", "KS", LatLng{38.8792, -99.3268}},
	{"Dodge City", "KS", LatLng{37.7528, -100.0171}}, {"Garden City", "KS", LatLng{37.9717, -100.8727}},
	{"Emporia", "KS", LatLng{38.4039, -96.1817}}, {"Pittsburg", "KS", LatLng{37.4109, -94.7049}},
	{"St. Louis", "MO", LatLng{38.6270, -90.1994}}, {"Columbia", "MO", LatLng{38.9517, -92.3341}},
	{"Springfield", "MO", LatLng{37.2090, -93.2923}}, {"Branson", "MO", LatLng{36.6437, -93.2185}},
	{"Joplin", "MO", LatLng{37.0842, -94.5133}}, {"St. Joseph", "MO", LatLng{39.7675, -94.8467}},
	{"Jefferson City", "MO", LatLng{38.5767, -92.1735}}, {"Osage Beach", "MO", LatLng{38.1503, -92.6179}},
	{"Lake of the Ozarks", "MO", LatLng{38.1503, -92.6179}}, {"Hannibal", "MO", LatLng{39.7084, -91.3585}},
	{"Cape Girardeau", "MO", LatLng{37.3059, -89.5181}}, {"Independence", "MO", LatLng{39.0911, -94.4155}},
	// Plains / Midwest
	{"Omaha", "NE", LatLng{41.2565, -95.9345}}, {"Lincoln", "NE", LatLng{40.8136, -96.7026}},
	{"Kearney", "NE", LatLng{40.6993, -99.0817}}, {"North Platte", "NE", LatLng{41.1403, -100.7601}},
	{"Scottsbluff", "NE", LatLng{41.8666, -103.6672}},
	{"Des Moines", "IA", LatLng{41.5868, -93.6250}}, {"Iowa City", "IA", LatLng{41.6611, -91.5302}},
	{"Cedar Rapids", "IA", LatLng{41.9779, -91.6656}}, {"Davenport", "IA", LatLng{41.5236, -90.5776}},
	{"Dubuque", "IA", LatLng{42.5006, -90.6646}}, {"Sioux City", "IA", LatLng{42.4963, -96.4049}},
	{"Ames", "IA", LatLng{42.0308, -93.6319}},
	{"Oklahoma City", "OK", LatLng{35.4676, -97.5164}}, {"Tulsa", "OK", LatLng{36.1540, -95.9928}},
	{"Broken Bow", "OK", LatLng{34.0293, -94.7391}}, {"Norman", "OK", LatLng{35.2226, -97.4395}},
	{"Little Rock", "AR", LatLng{34.7465, -92.2896}}, {"Hot Springs", "AR", LatLng{34.5037, -93.0552}},
	{"Eureka Springs", "AR", LatLng{36.4012, -93.7380}}, {"Bentonville", "AR", LatLng{36.3729, -94.2088}},
	{"Fayetteville", "AR", LatLng{36.0626, -94.1574}},
	{"Minneapolis", "MN", LatLng{44.9778, -93.2650}}, {"St. Paul", "MN", LatLng{44.9537, -93.0900}},
	{"Duluth", "MN", LatLng{46.7867, -92.1005}},
	{"Sioux Falls", "SD", LatLng{43.5446, -96.7311}}, {"Rapid City", "SD", LatLng{44.0805, -103.2310}},
	{"Fargo", "ND", LatLng{46.8772, -96.7898}}, {"Bismarck", "ND", LatLng{46.8083, -100.7837}},
	{"Chicago", "IL", LatLng{41.8781, -87.6298}}, {"Springfield", "IL", LatLng{39.7817, -89.6501}},
	{"Peoria", "IL", LatLng{40.6936, -89.5890}}, {"Champaign", "IL", LatLng{40.1164, -88.2434}},
	{"Galena", "IL", LatLng{42.4167, -90.4290}}, {"Rockford", "IL", LatLng{42.2711, -89.0940}},
	{"Milwaukee", "WI", LatLng{43.0389, -87.9065}}, {"Madison", "WI", LatLng{43.0731, -89.4012}},
	{"Green Bay", "WI", LatLng{44.5133, -88.0133}}, {"Wisconsin Dells", "WI", LatLng{43.6275, -89.7710}},
	{"Door County", "WI", LatLng{44.8342, -87.3770}}, {"Lake Geneva", "WI", LatLng{42.5917, -88.4334}},
	{"Indianapolis", "IN", LatLng{39.7684, -86.1581}}, {"Bloomington", "IN", LatLng{39.1653, -86.5264}},
	{"Fort Wayne", "IN", LatLng{41.0793, -85.1394}}, {"South Bend", "IN", LatLng{41.6764, -86.2520}},
	{"Indiana Dunes", "IN", LatLng{41.6533, -87.0524}},
	{"Detroit", "MI", LatLng{42.3314, -83.0458}}, {"Ann Arbor", "MI", LatLng{42.2808, -83.7430}},
	{"Grand Rapids", "MI", LatLng{42.9634, -85.6681}}, {"Traverse City", "MI", LatLng{44.7631, -85.6206}},
	{"Mackinac Island", "MI", LatLng{45.8492, -84.6189}}, {"Lansing", "MI", LatLng{42.7325, -84.5555}},
	{"Columbus", "OH", LatLng{39.9612, -82.9988}}, {"Cincinnati", "OH", LatLng{39.1031, -84.5120}},
	{"Cleveland", "OH", LatLng{41.4993, -81.6944}}, {"Dayton", "OH", LatLng{39.7589, -84.1916}},
	{"Toledo", "OH", LatLng{41.6528, -83.5379}}, {"Hocking Hills", "OH", LatLng{39.5400, -82.4071}},
	{"Louisville", "KY", LatLng{38.2527, -85.7585}}, {"Lexington", "KY", LatLng{38.0406, -84.5037}},
	{"Red River Gorge", "KY", LatLng{37.7912, -83.6866}}, {"Mammoth Cave", "KY", LatLng{37.1862, -86.1005}},
	{"Nashville", "TN", LatLng{36.1627, -86.7816}}, {"Memphis", "TN", LatLng{35.1495, -90.0490}},
	{"Knoxville", "TN", LatLng{35.9606, -83.9207}}, {"Chattanooga", "TN", LatLng{35.0456, -85.3097}},
	{"Gatlinburg", "TN", LatLng{35.7143, -83.5102}}, {"Pigeon Forge", "TN", LatLng{35.7884, -83.5543}},
	// Mountain West
	{"Denver", "CO", LatLng{39.7392, -104.9903}}, {"Boulder", "CO", LatLng{40.0150, -105.2705}},
	{"Colorado Springs", "CO", LatLng{38.8339, -104.8214}}, {"Fort Collins", "CO", LatLng{40.5853, -105.0844}},
	{"Estes Park", "CO", LatLng{40.3772, -105.5217}}, {"Aspen", "CO", LatLng{39.1911, -106.8175}},
	{"Vail", "CO", LatLng{39.6403, -106.3742}}, {"Breckenridge", "CO", LatLng{39.4817, -106.0384}},
	{"Steamboat Springs", "CO", LatLng{40.4850, -106.8317}}, {"Durango", "CO", LatLng{37.2753, -107.8801}},
	{"Grand Junction", "CO", LatLng{39.0639, -108.5506}}, {"Pueblo", "CO", LatLng{38.2544, -104.6091}},
	{"Telluride", "CO", LatLng{37.9375, -107.8123}}, {"Glenwood Springs", "CO", LatLng{39.5505, -107.3248}},
	{"Rocky Mountain National Park", "CO", LatLng{40.3428, -105.6836}}, {"Great Sand Dunes", "CO", LatLng{37.7916, -105.5943}},
	{"Mesa Verde", "CO", LatLng{37.2309, -108.4618}},
	{"Cheyenne", "WY", LatLng{41.1400, -104.8202}}, {"Casper", "WY", LatLng{42.8501, -106.3252}},
	{"Jackson", "WY", LatLng{43.4799, -110.7624}}, {"Jackson Hole", "WY", LatLng{43.4799, -110.7624}},
	{"Yellowstone", "WY", LatLng{44.4280, -110.5885}}, {"Grand Teton", "WY", LatLng{43.7904, -110.6818}},
	{"Cody", "WY", LatLng{44.5263, -109.0565}},
	{"Bozeman", "MT", LatLng{45.6770, -111.0429}}, {"Missoula", "MT", LatLng{46.8721, -113.9940}},
	{"Billings", "MT", LatLng{45.7833, -108.5007}}, {"Glacier National Park", "MT", LatLng{48.7596, -113.7870}},
	{"Whitefish", "MT", LatLng{48.4111, -114.3376}},
	{"Mount Rushmore", "SD", LatLng{43.8791, -103.4591}}, {"Badlands", "SD", LatLng{43.8554, -102.3397}},
	{"Custer", "SD", LatLng{43.7666, -103.5988}}, {"Deadwood", "SD", LatLng{44.3767, -103.7296}},
	{"Salt Lake City", "UT", LatLng{40.7608, -111.8910}}, {"Park City", "UT", LatLng{40.6461, -111.4980}},
	{"Moab", "UT", LatLng{38.5733, -109.5498}}, {"Arches", "UT", LatLng{38.7331, -109.5925}},
	{"Zion", "UT", LatLng{37.2982, -113.0263}}, {"Bryce Canyon", "UT", LatLng{37.5930, -112.1871}},
	{"Canyonlands", "UT", LatLng{38.3269, -109.8783}}, {"Capitol Reef", "UT", LatLng{38.3670, -111.2615}},
	{"St. George", "UT", LatLng{37.0965, -113.5684}},
	{"Boise", "ID", LatLng{43.6150, -116.2023}}, {"Sun Valley", "ID", LatLng{43.6971, -114.3517}},
	{"Coeur d'Alene", "ID", LatLng{47.6777, -116.7805}},
	{"Albuquerque", "NM", LatLng{35.0844, -106.6504}}, {"Santa Fe", "NM", LatLng{35.6870, -105.9378}},
	{"Taos", "NM", LatLng{36.4072, -105.5731}}, {"Las Cruces", "NM", LatLng{32.3199, -106.7637}},
	{"White Sands", "NM", LatLng{32.7872, -106.3257}}, {"Carlsbad Caverns", "NM", LatLng{32.1479, -104.5567}},
	{"Phoenix", "AZ", LatLng{33.4484, -112.0740}}, {"Scottsdale", "AZ", LatLng{33.4942, -111.9261}},
	{"Tucson", "AZ", LatLng{32.2226, -110.9747}}, {"Flagstaff", "AZ", LatLng{35.1983, -111.6513}},
	{"Sedona", "AZ", LatLng{34.8697, -111.7610}}, {"Grand Canyon", "AZ", LatLng{36.1069, -112.1129}},
	{"Page", "AZ", LatLng{36.9147, -111.4558}}, {"Petrified Forest", "AZ", LatLng{34.9100, -109.8068}},
	{"Las Vegas", "NV", LatLng{36.1699, -115.1398}}, {"Reno", "NV", LatLng{39.5296, -119.8138}},
	{"Lake Tahoe", "CA", LatLng{39.0968, -120.0324}}, {"South Lake Tahoe", "CA", LatLng{38.9399, -119.9772}},
	// Texas / South
	{"Dallas", "TX", LatLng{32.7767, -96.7970}}, {"Fort Worth", "TX", LatLng{32.7555, -97.3308}},
	{"Austin", "TX", LatLng{30.2672, -97.7431}}, {"Houston", "TX", LatLng{29.7604, -95.3698}},
	{"San Antonio", "TX", LatLng{29.4241, -98.4936}}, {"El Paso", "TX", LatLng{31.7619, -106.4850}},
	{"Galveston", "TX", LatLng{29.3013, -94.7977}}, {"Corpus Christi", "TX", LatLng{27.8006, -97.3964}},
	{"South Padre Island", "TX", LatLng{26.1118, -97.1681}}, {"Waco", "TX", LatLng{31.5493, -97.1467}},
	{"Fredericksburg", "TX", LatLng{30.2752, -98.8720}}, {"Amarillo", "TX", LatLng{35.2220, -101.8313}},
	{"Lubbock", "TX", LatLng{33.5779, -101.8552}}, {"Wichita Falls", "TX", LatLng{33.9137, -98.4934}},
	{"Marfa", "TX", LatLng{30.3085, -104.0205}}, {"Big Bend", "TX", LatLng{29.1275, -103.2425}},
	{"New Orleans", "LA", LatLng{29.9511, -90.0715}}, {"Baton Rouge", "LA", LatLng{30.4515, -91.1871}},
	{"Lafayette", "LA", LatLng{30.2241, -92.0198}}, {"Shreveport", "LA", LatLng{32.5252, -93.7502}},
	{"Jackson", "MS", LatLng{32.2988, -90.1848}}, {"Biloxi", "MS", LatLng{30.3960, -88.8853}},
	{"Mobile", "AL", LatLng{30.6954, -88.0399}}, {"Gulf Shores", "AL", LatLng{30.2460, -87.7008}},
	{"Birmingham", "AL", LatLng{33.5186, -86.8104}}, {"Huntsville", "AL", LatLng{34.7304, -86.5861}},
	{"Montgomery", "AL", LatLng{32.3668, -86.3000}},
	{"Atlanta", "GA", LatLng{33.7490, -84.3880}}, {"Savannah", "GA", LatLng{32.0809, -81.0912}},
	{"Athens", "GA", LatLng{33.9519, -83.3576}}, {"Augusta", "GA", LatLng{33.4735, -82.0105}},
	{"Miami", "FL", LatLng{25.7617, -80.1918}}, {"Orlando", "FL", LatLng{28.5383, -81.3792}},
	{"Tampa", "FL", LatLng{27.9506, -82.4572}}, {"Jacksonville", "FL", LatLng{30.3322, -81.6557}},
	{"Fort Lauderdale", "FL", LatLng{26.1224, -80.1373}}, {"Key West", "FL", LatLng{24.5551, -81.7800}},
	{"St. Augustine", "FL", LatLng{29.9012, -81.3124}}, {"Daytona Beach", "FL", LatLng{29.2108, -81.0228}},
	{"Pensacola", "FL", LatLng{30.4213, -87.2169}}, {"Destin", "FL", LatLng{30.3935, -86.4958}},
	{"Panama City Beach", "FL", LatLng{30.1766, -85.8055}}, {"Sarasota", "FL", LatLng{27.3364, -82.5307}},
	{"Naples", "FL", LatLng{26.1420, -81.7948}}, {"Fort Myers", "FL", LatLng{26.6406, -81.8723}},
	{"West Palm Beach", "FL", LatLng{26.7153, -80.0534}}, {"Tallahassee", "FL", LatLng{30.4383, -84.2807}},
	{"Clearwater", "FL", LatLng{27.9659, -82.8001}}, {"Everglades", "FL", LatLng{25.2866, -80.8987}},
	{"Charlotte", "NC", LatLng{35.2271, -80.8431}}, {"Raleigh", "NC", LatLng{35.7796, -78.6382}},
	{"Asheville", "NC", LatLng{35.5951, -82.5515}}, {"Wilmington", "NC", LatLng{34.2257, -77.9447}},
	{"Outer Banks", "NC", LatLng{35.9574, -75.6240}}, {"Durham", "NC", LatLng{35.9940, -78.8986}},
	{"Great Smoky Mountains", "TN", LatLng{35.6532, -83.5070}},
	{"Charleston", "SC", LatLng{32.7765, -79.9311}}, {"Myrtle Beach", "SC", LatLng{33.6891, -78.8867}},
	{"Greenville", "SC", LatLng{34.8526, -82.3940}}, {"Columbia", "SC", LatLng{34.0007, -81.0348}},
	{"Hilton Head", "SC", LatLng{32.2163, -80.7526}},
	// East
	{"New York", "NY", LatLng{40.7128, -74.0060}}, {"Buffalo", "NY", LatLng{42.8864, -78.8784}},
	{"Niagara Falls", "NY", LatLng{43.0962, -79.0377}}, {"Rochester", "NY", LatLng{43.1566, -77.6088}}, {"Rochester", "MN", LatLng{44.0121, -92.4802}},
	{"Albany", "NY", LatLng{42.6526, -73.7562}}, {"Syracuse", "NY", LatLng{43.0481, -76.1474}},
	{"Lake Placid", "NY", LatLng{44.2795, -73.9799}}, {"Ithaca", "NY", LatLng{42.4440, -76.5019}},
	{"Boston", "MA", LatLng{42.3601, -71.0589}}, {"Cape Cod", "MA", LatLng{41.6525, -70.2881}},
	{"Salem", "MA", LatLng{42.5195, -70.8967}}, {"Providence", "RI", LatLng{41.8240, -71.4128}},
	{"Newport", "RI", LatLng{41.4901, -71.3128}}, {"Hartford", "CT", LatLng{41.7658, -72.6734}},
	{"Burlington", "VT", LatLng{44.4759, -73.2121}}, {"Stowe", "VT", LatLng{44.4654, -72.6874}},
	{"Bar Harbor", "ME", LatLng{44.3876, -68.2039}},
	{"Acadia", "ME", LatLng{44.3386, -68.2733}}, {"Portsmouth", "NH", LatLng{43.0718, -70.7626}},
	{"Philadelphia", "PA", LatLng{39.9526, -75.1652}}, {"Pittsburgh", "PA", LatLng{40.4406, -79.9959}},
	{"Gettysburg", "PA", LatLng{39.8309, -77.2311}}, {"Hershey", "PA", LatLng{40.2859, -76.6502}},
	{"Lancaster", "PA", LatLng{40.0379, -76.3055}}, {"Erie", "PA", LatLng{42.1292, -80.0851}},
	{"Newark", "NJ", LatLng{40.7357, -74.1724}}, {"Atlantic City", "NJ", LatLng{39.3643, -74.4229}},
	{"Cape May", "NJ", LatLng{38.9351, -74.9060}},
	{"Washington", "DC", LatLng{38.9072, -77.0369}}, {"Washington DC", "DC", LatLng{38.9072, -77.0369}},
	{"Baltimore", "MD", LatLng{39.2904, -76.6122}}, {"Annapolis", "MD", LatLng{38.9784, -76.4922}},
	{"Ocean City", "MD", LatLng{38.3365, -75.0849}},
	{"Richmond", "VA", LatLng{37.5407, -77.4360}}, {"Virginia Beach", "VA", LatLng{36.8529, -75.9780}},
	{"Charlottesville", "VA", LatLng{38.0293, -78.4767}}, {"Shenandoah", "VA", LatLng{38.2928, -78.6796}},
	{"Norfolk", "VA", LatLng{36.8508, -76.2859}}, {"Roanoke", "VA", LatLng{37.2710, -79.9414}},
	// West coast
	{"Los Angeles", "CA", LatLng{34.0522, -118.2437}}, {"San Diego", "CA", LatLng{32.7157, -117.1611}},
	{"San Francisco", "CA", LatLng{37.7749, -122.4194}}, {"San Jose", "CA", LatLng{37.3382, -121.8863}},
	{"Sacramento", "CA", LatLng{38.5816, -121.4944}}, {"Santa Barbara", "CA", LatLng{34.4208, -119.6982}},
	{"Monterey", "CA", LatLng{36.6002, -121.8947}}, {"Napa", "CA", LatLng{38.2975, -122.2869}},
	{"Palm Springs", "CA", LatLng{33.8303, -116.5453}}, {"Santa Cruz", "CA", LatLng{36.9741, -122.0308}},
	{"Yosemite", "CA", LatLng{37.8651, -119.5383}}, {"Joshua Tree", "CA", LatLng{33.8734, -115.9010}},
	{"Death Valley", "CA", LatLng{36.5323, -116.9325}}, {"Sequoia", "CA", LatLng{36.4864, -118.5658}},
	{"Big Sur", "CA", LatLng{36.2704, -121.8081}}, {"Anaheim", "CA", LatLng{33.8366, -117.9143}},
	{"Portland", "OR", LatLng{45.5152, -122.6784}}, {"Portland", "ME", LatLng{43.6591, -70.2568}}, {"Bend", "OR", LatLng{44.0582, -121.3153}},
	{"Eugene", "OR", LatLng{44.0521, -123.0868}}, {"Crater Lake", "OR", LatLng{42.8684, -122.1685}},
	{"Seattle", "WA", LatLng{47.6062, -122.3321}}, {"Spokane", "WA", LatLng{47.6588, -117.4260}},
	{"Olympic National Park", "WA", LatLng{47.8021, -123.6044}}, {"Mount Rainier", "WA", LatLng{46.8800, -121.7269}},
	{"Leavenworth", "WA", LatLng{47.5962, -120.6615}},
	{"Honolulu", "HI", LatLng{21.3099, -157.8581}}, {"Maui", "HI", LatLng{20.7984, -156.3319}},
	{"Anchorage", "AK", LatLng{61.2181, -149.9003}}, {"Denali", "AK", LatLng{63.1148, -151.1926}},
}

var stateNames = map[string]string{
	"AL": "alabama", "AK": "alaska", "AZ": "arizona", "AR": "arkansas", "CA": "california", "CO": "colorado",
	"CT": "connecticut", "DE": "delaware", "DC": "district of columbia", "FL": "florida", "GA": "georgia",
	"HI": "hawaii", "ID": "idaho", "IL": "illinois", "IN": "indiana", "IA": "iowa", "KS": "kansas",
	"KY": "kentucky", "LA": "louisiana", "ME": "maine", "MD": "maryland", "MA": "massachusetts",
	"MI": "michigan", "MN": "minnesota", "MS": "mississippi", "MO": "missouri", "MT": "montana",
	"NE": "nebraska", "NV": "nevada", "NH": "new hampshire", "NJ": "new jersey", "NM": "new mexico",
	"NY": "new york", "NC": "north carolina", "ND": "north dakota", "OH": "ohio", "OK": "oklahoma",
	"OR": "oregon", "PA": "pennsylvania", "RI": "rhode island", "SC": "south carolina", "SD": "south dakota",
	"TN": "tennessee", "TX": "texas", "UT": "utah", "VT": "vermont", "VA": "virginia", "WA": "washington",
	"WV": "west virginia", "WI": "wisconsin", "WY": "wyoming",
}

// StateCode turns "Kansas" into "KS"; unknown names come back unchanged.
func StateCode(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	for code, full := range stateNames {
		if full == n || strings.EqualFold(code, n) {
			return code
		}
	}
	return name
}

// normalize folds a place string so "St. Louis, MO" == "st louis mo".
func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer(".", "", ",", " ", "'", "'", "’", "'").Replace(s)
	words := strings.Fields(s)
	// drop country suffixes and the word "national park" noise
	for len(words) > 0 {
		last := words[len(words)-1]
		if last == "usa" || last == "us" {
			words = words[:len(words)-1]
			continue
		}
		if len(words) >= 2 && words[len(words)-2]+" "+last == "united states" {
			words = words[:len(words)-2]
			continue
		}
		break
	}
	return strings.Join(words, " ")
}

type gazIndex struct {
	exact map[string]gazEntry // every accepted spelling → entry
	names []gazEntry          // longest name first, for substring fallback
}

var gaz = buildGazIndex()

func buildGazIndex() gazIndex {
	idx := gazIndex{exact: map[string]gazEntry{}}
	add := func(k string, e gazEntry) {
		if _, dup := idx.exact[k]; !dup {
			idx.exact[k] = e
		}
	}
	for _, e := range gazetteer {
		n := normalize(e.name)
		st := strings.ToLower(e.state)
		add(n, e) // first entry with a bare name wins (Kansas City MO before KS, Portland OR before ME)
		add(n+" "+st, e)
		add(n+" "+stateNames[e.state], e)
		if strings.Contains(n, "national park") {
			add(strings.TrimSpace(strings.TrimSuffix(n, "national park")), e)
		} else if !strings.Contains(n, "city") {
			add(n+" national park", e)
		}
	}
	idx.names = append(idx.names, gazetteer...)
	sort.SliceStable(idx.names, func(i, j int) bool { return len(idx.names[i].name) > len(idx.names[j].name) })
	return idx
}

func (e gazEntry) placemark() Placemark {
	return Placemark{Label: e.name + ", " + e.state, Name: e.name, Region: e.state, Country: "US", LatLng: e.ll, Source: "gazetteer", Kind: "city"}
}

// lookupGazetteer returns an exact match for a typed place, if any.
func lookupGazetteer(q string) (Placemark, bool) {
	if e, ok := gaz.exact[normalize(q)]; ok {
		return e.placemark(), true
	}
	return Placemark{}, false
}

// lookupGazetteerLoose finds a known place mentioned anywhere in the text
// ("downtown Kansas City", "near Denver"). Used only when live geocoding is
// unavailable, since it is happy to match "Nashville, Indiana" to Tennessee.
func lookupGazetteerLoose(q string) (Placemark, bool) {
	n := " " + normalize(q) + " "
	for _, e := range gaz.names {
		if strings.Contains(n, " "+normalize(e.name)+" ") {
			return e.placemark(), true
		}
	}
	return Placemark{}, false
}

// suggestGazetteer offers prefix matches for an autocomplete box when the
// live suggester is unreachable.
func suggestGazetteer(q string, limit int) []Placemark {
	n := normalize(q)
	if n == "" {
		return nil
	}
	out := []Placemark{}
	seen := map[string]bool{}
	for _, e := range gazetteer {
		en := normalize(e.name)
		if strings.HasPrefix(en, n) || strings.HasPrefix(en+" "+strings.ToLower(e.state), n) {
			p := e.placemark()
			if !seen[p.Label] {
				seen[p.Label] = true
				out = append(out, p)
			}
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

/**
 * A photo for each kind of place, so the moment someone names a destination
 * the trip card shows the right sort of scenery: a skyline for a city, peaks
 * for a mountain town, a shoreline for a lake.
 *
 * Photos are free-licensed Wikimedia Commons images, hotlinked at 1280px
 * (one of the widths Wikimedia pre-renders). Each carries its author and
 * licence so the UI can credit them.
 */

export type PlaceKind = 'city' | 'town' | 'mountains' | 'lake' | 'coast' | 'park' | 'forest' | 'desert' | 'canyon' | 'falls' | 'prairie' | 'island'

export type PlacePhoto = { src: string; alt: string; author: string; license: string; file: string }

const commons = (hash: string, file: string) => `https://upload.wikimedia.org/wikipedia/commons/thumb/${hash}/${file}/1280px-${file}`

export const PLACE_PHOTOS: Record<PlaceKind, PlacePhoto> = {
  city: { src: commons('a/a5', 'Chicago_River_ferry_b.jpg'), alt: 'A river running between downtown towers', author: 'John Picken', license: 'CC BY 2.0', file: 'Chicago_River_ferry_b.jpg' },
  town: { src: commons('9/92', 'Galena%2C_Illinois.jpg'), alt: 'A historic main street lined with brick storefronts', author: 'rboed', license: 'CC BY 2.0', file: 'Galena,_Illinois.jpg' },
  mountains: { src: commons('5/53', 'Maroon_Bells_%2811553%29a.jpg'), alt: 'Snow-dusted peaks above an alpine lake', author: 'Rhododendrites', license: 'CC BY-SA 4.0', file: 'Maroon_Bells_(11553)a.jpg' },
  lake: { src: commons('d/da', 'Tahoe_North_Shore_from_the_East_Shore.jpg'), alt: 'A clear blue lake ringed by pines', author: 'Lara Farhadi', license: 'CC BY 2.0', file: 'Tahoe_North_Shore_from_the_East_Shore.jpg' },
  coast: { src: commons('0/09', 'Central_Californian_Coastline%2C_Big_Sur_-_May_2013.jpg'), alt: 'Cliffs dropping into a turquoise sea', author: 'Diliff', license: 'CC BY-SA 3.0', file: 'Central_Californian_Coastline,_Big_Sur_-_May_2013.jpg' },
  park: { src: commons('e/ea', 'Half_Dome_with_Eastern_Yosemite_Valley_%2850MP%29.jpg'), alt: 'A granite dome over a forested valley', author: 'Thomas Wolf', license: 'CC BY-SA 3.0', file: 'Half_Dome_with_Eastern_Yosemite_Valley_(50MP).jpg' },
  forest: { src: commons('d/de', 'Redwood_National_Park%2C_fog_in_the_forest.jpg'), alt: 'Fog drifting through tall redwoods', author: 'Michael Schweppe', license: 'CC BY-SA 2.0', file: 'Redwood_National_Park,_fog_in_the_forest.jpg' },
  desert: { src: commons('7/78', 'Monument_Valley%2C_Utah%2C_USA_%2823611451292%29.jpg'), alt: 'Red sandstone buttes under a wide sky', author: 'Domenico Convertini', license: 'CC BY-SA 2.0', file: 'Monument_Valley,_Utah,_USA_(23611451292).jpg' },
  canyon: { src: commons('3/31', 'Canyon_River_Tree_%28165872763%29.jpeg'), alt: 'A river winding through a deep canyon', author: 'Lennart Sikkema', license: 'CC BY 3.0', file: 'Canyon_River_Tree_(165872763).jpeg' },
  falls: { src: commons('a/ab', '3Falls_Niagara.jpg'), alt: 'Waterfalls throwing up mist', author: 'Saffron Blaze', license: 'CC BY-SA 3.0', file: '3Falls_Niagara.jpg' },
  prairie: { src: commons('f/fe', 'Flint_hills_kansas.jpg'), alt: 'Rolling grassland under a big sky', author: 'Non-dropframe', license: 'CC BY-SA 3.0', file: 'Flint_hills_kansas.jpg' },
  island: { src: commons('c/c9', 'Fish_Creek%2C_Wisconsin.jpg'), alt: 'A harbor town on a wooded shore', author: 'DNerenhausen', license: 'CC0', file: 'Fish_Creek,_Wisconsin.jpg' },
}

export const commonsFileURL = (p: PlacePhoto) => `https://commons.wikimedia.org/wiki/File:${encodeURIComponent(p.file)}`

/** Words in a place name that tell you what it is, checked in order. */
const RULES: [PlaceKind, RegExp][] = [
  ['falls', /\b(falls|waterfall)\b/i],
  ['canyon', /\b(canyon|gorge|arches|bryce|zion)\b/i],
  ['desert', /\b(desert|dunes|moab|sedona|joshua tree|death valley|monument valley|mojave|sonoran)\b/i],
  ['island', /\b(island|isle|key|keys|cape cod|nantucket|mackinac|door county)\b/i],
  ['coast', /\b(beach|coast|shore|shores|bay|cove|harbor|harbour|gulf|seaside|oceanside|big sur|outer banks|myrtle|galveston|destin|monterey)\b/i],
  ['lake', /\b(lake|lakes|reservoir|tahoe|ozarks|okoboji|geneva|finger lakes|lake of the)\b/i],
  ['mountains', /\b(mountain|mountains|mount|mt\.?|peak|peaks|summit|alpine|aspen|vail|breckenridge|estes park|telluride|jackson hole|gatlinburg|pigeon forge|steamboat|taos|rockies|smokies|ski)\b/i],
  ['forest', /\b(forest|woods|redwood|redwoods|sequoia|rainforest|boundary waters|northwoods)\b/i],
  ['prairie', /\b(prairie|plains|flint hills|badlands|ranch|grassland)\b/i],
  ['park', /\b(national park|state park|park|preserve|wilderness|monument|yellowstone|yosemite|glacier|grand teton|great smoky)\b/i],
]

const BIG_CITIES = /\b(new york|nyc|los angeles|chicago|houston|phoenix|philadelphia|san antonio|san diego|dallas|austin|san jose|san francisco|seattle|denver|boston|nashville|atlanta|miami|las vegas|minneapolis|st\.? louis|kansas city|omaha|oklahoma city|tulsa|milwaukee|indianapolis|columbus|cincinnati|cleveland|pittsburgh|detroit|memphis|new orleans|portland|salt lake city|washington|baltimore|charlotte|orlando|tampa|toronto|montreal|vancouver|london|paris)\b/i

/**
 * Pick the photo kind for a destination from what the geocoder said it is
 * (`kind`) and what its name gives away. Nature words win over the geocoder's
 * "town" because a town called Estes Park is really about the mountains.
 */
export function placeKind(name: string, kind?: string): PlaceKind {
  const text = name.trim()
  for (const [k, re] of RULES) if (re.test(text)) return k
  if (BIG_CITIES.test(text)) return 'city'
  switch (kind) {
    case 'city': return 'city'
    case 'town': case 'village': case 'locality': return 'town'
    case 'island': return 'island'
    case 'park': return 'park'
  }
  // A bare name with a state ("Lawrence, KS") is most likely a town.
  return /,\s*[A-Z]{2}\b/.test(text) ? 'town' : 'city'
}

export const placePhoto = (name: string, kind?: string): PlacePhoto => PLACE_PHOTOS[placeKind(name, kind)]

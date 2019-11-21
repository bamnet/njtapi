package njtapi

// extraStations provides aliases for stations that have multiple names
// depending on which endpoint you are invoking and how it's feeling.
var extraStations = map[string][]string{
	"AM": {"Aberdeen-Matawan"},
	"CN": {"Convent Station"},
	"HI": {"Highland Avenue"},
	"JA": {"Jersey Avenue"},
	"HS": {"Montclair Heights"},
	"UV": {"Montclair State U"},
	"MS": {"Mountain Avenue"},
	"MT": {"Mountain Station"},
	"NY": {"New York Penn Station"},
	"ND": {"Newark Broad Street"},
	"NP": {"Newark Penn Station"},
	"NZ": {"North Elizabeth"},
	"PP": {"Point Pleasant Beach"},
	"PJ": {"Princeton Junction"},
	"SE": {"Secaucus Upper Lvl"},
	"TS": {"Secaucus Lower Lvl"},
	"UM": {"Upper Montclair"},
	"WG": {"Watchung Avenue"},
	"WT": {"Watsessing Avenue"},
}

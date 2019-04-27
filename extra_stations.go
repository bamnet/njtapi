package njtapi

// extraStations provides aliases for stations that have multiple names
// depending on which endpoint you are invoking and how it's feeling.
var extraStations = map[string][]string{
	"AM": []string{"Aberdeen-Matawan"},
	"CN": []string{"Convent Station"},
	"HI": []string{"Highland Avenue"},
	"JA": []string{"Jersey Avenue"},
	"HS": []string{"Montclair Heights"},
	"UV": []string{"Montclair State U"},
	"MS": []string{"Mountain Avenue"},
	"MT": []string{"Mountain Station"},
	"NY": []string{"New York Penn Station"},
	"ND": []string{"Newark Broad Street"},
	"NP": []string{"Newark Penn Station"},
	"NZ": []string{"North Elizabeth"},
	"PP": []string{"Point Pleasant Beach"},
	"PJ": []string{"Princeton Junction"},
	"SE": []string{"Secaucus Upper Lvl"},
	"UM": []string{"Upper Montclair"},
	"WG": []string{"Watchung Avenue"},
	"WT": []string{"Watsessing Avenue"},
}

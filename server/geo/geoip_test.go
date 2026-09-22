package geo

import (
	"testing"
)

func TestPredefinedCloudflare(t *testing.T) {
	gs := NewGeoService("")
	defer gs.Close()

	loc := gs.Lookup("172.64.80.1")
	if loc.Country != "美国" {
		t.Errorf("Expected country '美国', got '%s'", loc.Country)
	}
	if loc.Longitude > 0 { // Should be negative (USA West Coast longitude -121.8863)
		t.Errorf("Expected negative longitude in USA for Cloudflare IP, got %f", loc.Longitude)
	}
	if loc.City == "美国" {
		t.Errorf("City should not be duplicated as '美国', got '%s'", loc.City)
	}
}

func TestChinaCityCoordsMatch(t *testing.T) {
	coord, found := matchCityCoord("泉州市", "晋江市")
	if !found {
		t.Fatalf("Expected to find coordinates for 晋江市")
	}
	// Jinjiang lng: 118.5755, lat: 24.8197
	if coord[0] < 118.0 || coord[0] > 119.0 || coord[1] < 24.0 || coord[1] > 25.0 {
		t.Errorf("Coordinates for 晋江市 out of expected range: %v", coord)
	}

	coordQZ, foundQZ := matchCityCoord("泉州市", "")
	if !foundQZ {
		t.Fatalf("Expected to find coordinates for 泉州市")
	}
	if coordQZ[0] < 118.0 || coordQZ[0] > 119.0 || coordQZ[1] < 24.0 || coordQZ[1] > 25.5 {
		t.Errorf("Coordinates for 泉州市 out of expected range: %v", coordQZ)
	}
}

func TestWorldCountryCoords(t *testing.T) {
	coord, found := matchWorldCountryCoord("美国")
	if !found {
		t.Fatalf("Expected to find coordinates for 美国")
	}
	if coord[0] > 0 {
		t.Errorf("Expected negative longitude for USA center, got %f", coord[0])
	}
}

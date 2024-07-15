package powerdns

import (
	"fmt"
	"math"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

var geoIPReader *maxminddb.Reader

func InitGeoIP(dbPath string) error {
	var err error
	geoIPReader, err = maxminddb.Open(dbPath)
	return err
}

func getClientCoordinates(ipStr string) (float64, float64, error) {
	if geoIPReader == nil {
		return 0, 0, fmt.Errorf("GeoIP database is not initialized")
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0, 0, fmt.Errorf("invalid IP address")
	}

	var record struct {
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
		} `maxminddb:"location"`
	}

	err := geoIPReader.Lookup(ip, &record)
	if err != nil {
		return 0, 0, err
	}

	return record.Location.Latitude, record.Location.Longitude, nil
}

func distance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	lat1 = lat1 * (math.Pi / 180.0)
	lat2 = lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

package reql

import (
	"fmt"
	"math"
)

// Point represents a geographic point
type Point struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Circle represents a geographic circle
type Circle struct {
	Center   Point   `json:"center"`
	Radius   float64 `json:"radius"` // in meters
	NumVerts int     `json:"num_verts,omitempty"`
}

// Line represents a geographic line
type Line struct {
	Points []Point `json:"points"`
}

// Polygon represents a geographic polygon
type Polygon struct {
	Points []Point `json:"points"`
}

// GeoOperation represents a geographic operation
type GeoOperation int

const (
	GeoDistance GeoOperation = iota
	GeoIntersects
	GeoIncludes
	GeoNear
)

// String returns the string representation of geo operation
func (g GeoOperation) String() string {
	switch g {
	case GeoDistance:
		return "distance"
	case GeoIntersects:
		return "intersects"
	case GeoIncludes:
		return "includes"
	case GeoNear:
		return "near"
	default:
		return "unknown"
	}
}

// Distance calculates the distance between two points in meters
func Distance(p1, p2 Point) float64 {
	const R = 6371000 // Earth radius in meters

	lat1Rad := p1.Latitude * math.Pi / 180
	lat2Rad := p2.Latitude * math.Pi / 180
	deltaLat := (p2.Latitude - p1.Latitude) * math.Pi / 180
	deltaLon := (p2.Longitude - p1.Longitude) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// Intersects checks if two geometries intersect
func Intersects(geom1, geom2 interface{}) bool {
	// Simple implementation for point-circle intersection
	switch g1 := geom1.(type) {
	case Point:
		switch g2 := geom2.(type) {
		case Circle:
			dist := Distance(g1, g2.Center)
			return dist <= g2.Radius
		case Point:
			return g1.Latitude == g2.Latitude && g1.Longitude == g2.Longitude
		}
	case Circle:
		switch g2 := geom2.(type) {
		case Point:
			dist := Distance(g2, g1.Center)
			return dist <= g1.Radius
		case Circle:
			dist := Distance(g1.Center, g2.Center)
			return dist <= (g1.Radius + g2.Radius)
		}
	}
	return false
}

// Includes checks if a geometry includes another
func Includes(geom1, geom2 interface{}) bool {
	// Simple implementation for circle-point inclusion
	switch g1 := geom1.(type) {
	case Circle:
		switch g2 := geom2.(type) {
		case Point:
			dist := Distance(g2, g1.Center)
			return dist <= g1.Radius
		}
	}
	return false
}

// Near finds points near a given point within a radius
func (e *Evaluator) Near(data []map[string]interface{}, pointField string, center Point, maxDist float64) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, doc := range data {
		if pointValue, ok := doc[pointField]; ok {
			if point, ok := pointValue.(Point); ok {
				dist := Distance(point, center)
				if dist <= maxDist {
					// Add distance to document
					doc["dist"] = dist
					results = append(results, doc)
				}
			}
		}
	}

	return results, nil
}

// CreatePoint creates a point from coordinates
func CreatePoint(lat, lon float64) Point {
	return Point{
		Latitude:  lat,
		Longitude: lon,
	}
}

// CreateCircle creates a circle from center and radius
func CreateCircle(center Point, radius float64) Circle {
	return Circle{
		Center: center,
		Radius: radius,
	}
}

// CreateLine creates a line from points
func CreateLine(points []Point) Line {
	return Line{
		Points: points,
	}
}

// CreatePolygon creates a polygon from points
func CreatePolygon(points []Point) Polygon {
	return Polygon{
		Points: points,
	}
}

// ValidatePoint validates a point
func ValidatePoint(p Point) error {
	if p.Latitude < -90 || p.Latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if p.Longitude < -180 || p.Longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}

// ValidateCircle validates a circle
func ValidateCircle(c Circle) error {
	if err := ValidatePoint(c.Center); err != nil {
		return err
	}
	if c.Radius <= 0 {
		return fmt.Errorf("radius must be positive")
	}
	return nil
}

// ToRadians converts degrees to radians
func ToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// ToDegrees converts radians to degrees
func ToDegrees(radians float64) float64 {
	return radians * 180 / math.Pi
}

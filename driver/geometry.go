package driver

// GeometryPoint is the Go representation of a SurrealDB geometry point.
// Both drivers normalize points to this struct with Longitude (x) and
// Latitude (y) as float64 values.
type GeometryPoint struct {
	Longitude float64
	Latitude  float64
}

// GeometryLine is the Go representation of a SurrealDB geometry line
// (LineString). It is an ordered sequence of points.
type GeometryLine []GeometryPoint

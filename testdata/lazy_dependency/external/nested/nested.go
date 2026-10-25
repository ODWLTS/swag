package nested

// Detail is only reachable through external.Thing.
type Detail struct {
	Name string `json:"name"`
}

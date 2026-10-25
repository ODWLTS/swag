package api

import (
	"net/http"

	"github.com/ODWLTS/swag/testdata/lazy_dependency/external"
)

var _ external.Thing

// GetThing example
// @Summary get a thing
// @ID get_thing
// @Produce json
// @Success 200 {object} external.Thing
// @Router /testapi/lazy [get]
func GetThing(w http.ResponseWriter, r *http.Request) {
}

package main

// The handlers under api/ are parsed because they live in the search dir, but
// main does not import them, so the sibling external package they reference is
// neither walked nor in main's dependency set: it has to be loaded lazily.

// @title Swagger Example API
// @version 1.0
// @description Handlers reference types of a package that main does not depend on.
// @BasePath /v1
func main() {
}

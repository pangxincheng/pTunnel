package version

// Version is the version of the project.
// During the compilation process, this
// variable will be updated by the go compiler
var version = "v0.1.0"

func GetVersion() string {
	return version
}

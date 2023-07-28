package model

// Config is a data structure that represents a configuration entry.
// It contains information about the name, type, and data of the configuration.
type Config struct {
	Name string      // Name of the configuration entry.
	Data interface{} // The actual data of the configuration, which can be of any type.
}

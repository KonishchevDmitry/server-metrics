package cgroups

type ToNamedUsage interface {
	ToNamedUsage() []NamedUsage
}

type NamedUsage struct {
	Name  string
	Value *int64

	Monotonic    bool
	AllowedError int64
}

func MakeMonotonicNamedUsage(name string, value *int64, allowedError int64) NamedUsage {
	return NamedUsage{
		Name:  name,
		Value: value,

		Monotonic:    true,
		AllowedError: allowedError,
	}
}

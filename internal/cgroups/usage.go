package cgroups

type ToNamedUsage interface {
	ToNamedUsage() []NamedUsage
}

type NamedUsage struct {
	Name      string
	Value     *int64
	Precision int64
}

func MakeNamedUsage(name string, value *int64, precision int64) NamedUsage {
	return NamedUsage{
		Name:      name,
		Value:     value,
		Precision: precision,
	}
}

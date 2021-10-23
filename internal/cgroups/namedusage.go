package cgroups

import (
	"golang.org/x/xerrors"
)

type ToNamedUsage interface {
	ToNamedUsage() []NamedUsage
}

type ToRootUsage interface {
	ToRootUsage() (root ToNamedUsage, children ToNamedUsage)
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

func AddUsage(total ToNamedUsage, other ToNamedUsage) {
	totalUsages := total.ToNamedUsage()
	for index, usage := range other.ToNamedUsage() {
		*totalUsages[index].Value += *usage.Value
	}
}

func CalculateRootGroupUsage(netRootUsage ToNamedUsage, current ToRootUsage, previous ToRootUsage) error {
	// We do this manual racy calculations as the best effort: on my server a get the following results:
	//
	// /sys/fs/cgroup# grep user_usec cpu.stat | cut -d ' ' -f 2
	// 46283360000
	// /sys/fs/cgroup# bc <<< $(grep user_usec */cpu.stat | cut -d ' ' -f 2 | tr '\n' '+' | sed 's/\+$//')
	// 51792306852
	//
	// As you can see, about 10% of root CPU usage is lost somewhere. So as a workaround we manually calculate diffs and
	// hope that they will be precise enough.

	currentRootUsages, currentChildrenUsages := current.ToRootUsage()
	previousRootUsages, previousChildrenUsages := previous.ToRootUsage()

	rootDiffs, err := diffUsage(currentRootUsages, previousRootUsages)
	if err != nil {
		return err
	}

	childrenDiffs, err := diffUsage(currentChildrenUsages, previousChildrenUsages)
	if err != nil {
		return err
	}

	for index, netRootUsage := range netRootUsage.ToNamedUsage() {
		rootDiff := *rootDiffs[index].Value
		childrenDiff := *childrenDiffs[index].Value
		if rootDiff > childrenDiff {
			*netRootUsage.Value += rootDiff - childrenDiff
		}
	}

	return nil
}

func diffUsage(current ToNamedUsage, previous ToNamedUsage) ([]NamedUsage, error) {
	currentUsages := current.ToNamedUsage()
	previousUsages := previous.ToNamedUsage()
	diffUsages := make([]NamedUsage, 0, len(currentUsages))

	for index, current := range currentUsages {
		diff := *current.Value - *previousUsages[index].Value
		if diff < 0 {
			return nil, xerrors.Errorf("Got a negative %s", current.Name)
		}
		diffUsages = append(diffUsages, MakeNamedUsage(current.Name, &diff, current.Precision))
	}

	return diffUsages, nil
}

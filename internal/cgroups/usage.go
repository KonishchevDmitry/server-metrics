package cgroups

import (
	"fmt"
)

type ToUsage interface {
	ToUsage() []Usage
}

type ToRootUsage interface {
	ToRootUsage() (root ToUsage, children ToUsage)
}

type Usage struct {
	Name  string
	Value *int64
}

func MakeUsage(name string, value *int64) Usage {
	return Usage{
		Name:  name,
		Value: value,
	}
}

func AddUsage(total ToUsage, other ToUsage) {
	totalUsages := total.ToUsage()
	for index, usage := range other.ToUsage() {
		*totalUsages[index].Value += *usage.Value
	}
}

func CalculateRootGroupUsage(netRootUsage ToUsage, current ToRootUsage, previous ToRootUsage) error {
	// We do this manual racy calculations as the best effort: on my server I get the following results:
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

	for index, netRootUsage := range netRootUsage.ToUsage() {
		rootDiff := *rootDiffs[index].Value
		childrenDiff := *childrenDiffs[index].Value
		if rootDiff > childrenDiff {
			*netRootUsage.Value += rootDiff - childrenDiff
		}
	}

	return nil
}

func diffUsage(current ToUsage, previous ToUsage) ([]Usage, error) {
	currentUsages := current.ToUsage()
	previousUsages := previous.ToUsage()
	diffUsages := make([]Usage, 0, len(currentUsages))

	for index, current := range currentUsages {
		diff := *current.Value - *previousUsages[index].Value
		if diff < 0 {
			return nil, fmt.Errorf("Got a negative %s", current.Name)
		}
		diffUsages = append(diffUsages, MakeUsage(current.Name, &diff))
	}

	return diffUsages, nil
}

package kernel

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	logging "github.com/KonishchevDmitry/go-easy-logging"
)

type amdIOMMUErrorMatcher struct {
	regexp *regexp.Regexp
}

func newAMDIOMMUErrorMatcher() *amdIOMMUErrorMatcher {
	return &amdIOMMUErrorMatcher{regexp.MustCompile(
		`^kfd kfd: amdgpu: Failed to resume IOMMU for device ([a-f0-9:]+)$`,
	)}
}

// HP Proliant MicroServer Gen10 has a numerous bugs in IOMMU support. Ignore complains on them.
func (e *amdIOMMUErrorMatcher) match(ctx context.Context, messages []string) (int, []errorType) {
	matches := e.regexp.FindStringSubmatch(messages[0])
	if len(matches) == 0 {
		return 0, nil
	}

	count := 1

	expectedNext := fmt.Sprintf("kfd kfd: amdgpu: device %s NOT added due to errors", matches[1])
	if len(messages) > count && messages[count] == expectedNext {
		count++
	}

	logging.L(ctx).Infof("Ignoring IOMMU errors:\n%s", formatMessages(messages[:count]))
	return count, nil
}

type unexpectedNMIErrorMatcher struct {
}

func newUnexpectedNMIErrorMatcher() *unexpectedNMIErrorMatcher {
	return &unexpectedNMIErrorMatcher{}
}

func (e *unexpectedNMIErrorMatcher) match(ctx context.Context, messages []string) (int, []errorType) {
	if !strings.HasPrefix(messages[0], "Uhhuh. NMI received for unknown reason") {
		return 0, nil
	}

	count := 1

	expectedNext := "Dazed and confused, but trying to continue"
	if len(messages) > count && messages[count] == expectedNext {
		count++
	}

	logging.L(ctx).Warnf("Got an unexpected NMI:\n%s", formatMessages(messages[:count]))
	return count, []errorType{errorTypeUnexpectedNMI}
}

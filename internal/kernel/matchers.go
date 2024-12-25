package kernel

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	logging "github.com/KonishchevDmitry/go-easy-logging"
)

// HP Proliant MicroServer Gen10 has a numerous bugs in IOMMU support. Ignore complains on them.
type amdIOMMUErrorMatcher struct {
	regexp *regexp.Regexp
}

func newAMDIOMMUErrorMatcher() *amdIOMMUErrorMatcher {
	return &amdIOMMUErrorMatcher{regexp.MustCompile(
		`^kfd kfd: amdgpu: Failed to resume IOMMU for device ([a-f0-9:]+)$`,
	)}
}

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

// Virtualization issues on VPS
type hotplugInitializationErrorMatcher struct {
	prefix *regexp.Regexp
}

func newHotplugInitializationErrorMatcher() *hotplugInitializationErrorMatcher {
	return &hotplugInitializationErrorMatcher{regexp.MustCompile(`^shpchp \d+:\d+:\d+\.\d+: `)}
}

func (e *hotplugInitializationErrorMatcher) match(ctx context.Context, messages []string) (int, []errorType) {
	count := 0

	for {
		if match := e.prefix.FindStringSubmatch(messages[count]); len(match) == 0 {
			break
		} else if message := messages[count][len(match[0]):]; message != "pci_hp_register failed with error -16" {
			break
		}

		count++
		if count == len(messages) {
			break
		}

		if match := e.prefix.FindStringSubmatch(messages[count]); len(match) == 0 {
			break
		} else if message := messages[count][len(match[0]):]; message != "Slot initialization failed" {
			break
		}

		count++
		if count == len(messages) {
			break
		}
	}

	if count != 0 {
		logging.L(ctx).Infof("Ignoring device hotplug initialization errors:\n%s", formatMessages(messages[:count]))
	}

	return count, nil
}

type ubsanErrorMatcher struct {
	indexOutOfRangeRegexp *regexp.Regexp
}

func newUBSANErrorMatcher() *ubsanErrorMatcher {
	return &ubsanErrorMatcher{regexp.MustCompile(
		`^index \d+ is out of range for type '[^']+'$`,
	)}
}

func (e *ubsanErrorMatcher) match(ctx context.Context, messages []string) (int, []errorType) {
	if !strings.HasPrefix(messages[0], "UBSAN: array-index-out-of-bounds in ") {
		return 0, nil
	}

	count := 1
	if len(messages) > count && len(e.indexOutOfRangeRegexp.FindStringSubmatch(messages[count])) != 0 {
		count++
	}

	logging.L(ctx).Infof("Ignoring UBSAN errors:\n%s", formatMessages(messages[:count]))
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

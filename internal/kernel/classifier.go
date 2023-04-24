package kernel

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	logging "github.com/KonishchevDmitry/go-easy-logging"
)

type errorType string

const (
	errorTypeUnknown errorType = "unknown"
)

var errorTypes = []errorType{errorTypeUnknown}

func classify(ctx context.Context, messages []string) []errorType {
	var errors []errorType

	consume := func(count int) {
		messages = messages[count:]
	}

	var index int
	consumeUnknown := func() {
		if index != 0 {
			logging.L(ctx).Warnf("Got a kernel error:\n%s", formatMessages(messages[:index]))
			errors = append(errors, errorTypeUnknown)
			consume(index)
			index = 0
		}
	}

	for index < len(messages) {
		message := messages[index]

		if matches := amdIOMMUError.FindStringSubmatch(message); len(matches) != 0 {
			consumeUnknown()
			consume(handleAMDIOMMUError(ctx, messages, matches))
		} else {
			index++
		}
	}

	consumeUnknown()
	return errors
}

var amdIOMMUError = regexp.MustCompile(`^kfd kfd: amdgpu: Failed to resume IOMMU for device ([a-f0-9:]+)$`)

// HP Proliant MicroServer Gen10 has a numerous bugs in IOMMU support. Ignore complains on them.
func handleAMDIOMMUError(ctx context.Context, messages []string, matches []string) int {
	count := 1

	expectedNext := fmt.Sprintf("kfd kfd: amdgpu: device %s NOT added due to errors", matches[1])
	if len(messages) > count && messages[count] == expectedNext {
		count++
	}

	logging.L(ctx).Infof("Ignoring IOMMU errors:\n%s", formatMessages(messages[:count]))
	return count
}

func formatMessages(messages []string) string {
	return strings.Join(messages, "\n")
}

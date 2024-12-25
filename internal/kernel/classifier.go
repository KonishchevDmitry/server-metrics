package kernel

import (
	"context"
	"strings"

	logging "github.com/KonishchevDmitry/go-easy-logging"
	"github.com/klauspost/cpuid/v2"
)

type errorType string

const (
	errorTypeUnknown       errorType = "unknown"
	errorTypeUnexpectedNMI errorType = "unexpected-nmi"
)

var errorTypes = []errorType{
	errorTypeUnknown,
	errorTypeUnexpectedNMI,
}

type errorMatcher interface {
	match(ctx context.Context, messages []string) (int, []errorType)
}

type classifier struct {
	matchers []errorMatcher
}

func newClassifier(enableAll bool) *classifier {
	matchers := []errorMatcher{
		newAMDIOMMUErrorMatcher(),
		newMissingHardwareWatchdogMatcher(),
		newUBSANErrorMatcher(),
		newUnexpectedNMIErrorMatcher(),
	}

	if enableAll || cpuid.CPU.Has(cpuid.HYPERVISOR) {
		matchers = append(matchers, newHotplugInitializationErrorMatcher())
	}

	return &classifier{matchers: matchers}
}

func (c *classifier) classify(ctx context.Context, messages []string) []errorType {
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

MessageLoop:
	for index < len(messages) {
		toMatch := messages[index:]

		for _, matcher := range c.matchers {
			if count, types := matcher.match(ctx, toMatch); count != 0 {
				errors = append(errors, types...)
				consumeUnknown()
				consume(count)
				continue MessageLoop
			}
		}

		index++
	}

	consumeUnknown()
	return errors
}

func formatMessages(messages []string) string {
	return strings.Join(messages, "\n")
}

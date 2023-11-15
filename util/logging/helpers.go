package logging

import (
	"fmt"

	"github.com/go-logr/logr"
)

// LogAndWrapError logs error and returns a wrapped error.
func LogAndWrapError(log logr.Logger, msg string, err error, target interface{}) error {
	log.V(1).Info(msg, "error", err.Error())
	log.V(3).Error(err, msg, "target", target)

	return fmt.Errorf("%s: %w", msg, err)
}

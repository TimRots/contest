// Copyright (c) Facebook, Inc. and its affiliates.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package slowecho

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/facebookincubator/contest/pkg/cerrors"
	"github.com/facebookincubator/contest/pkg/event"
	"github.com/facebookincubator/contest/pkg/event/testevent"
	"github.com/facebookincubator/contest/pkg/logging"
	"github.com/facebookincubator/contest/pkg/target"
	"github.com/facebookincubator/contest/pkg/test"
)

// Name is the name used to look this plugin up.
var Name = "SlowEcho"

var log = logging.GetLogger("teststeps/" + strings.ToLower(Name))

// Events defines the events that a TestStep is allow to emit
var Events = []event.Name{}

// Step implements an echo-style printing plugin.
type Step struct {
}

// New initializes and returns a new EchoStep. It implements the TestStepFactory
// interface.
func New() test.TestStep {
	return &Step{}
}

// Load returns the name, factory and events which are needed to register the step.
func Load() (string, test.TestStepFactory, []event.Name) {
	return Name, New, Events
}

// Name returns the name of the Step
func (e *Step) Name() string {
	return Name
}

func sleepTime(secStr string) (time.Duration, error) {
	seconds, err := strconv.Atoi(secStr)
	if err != nil {
		return 0, err
	}
	if seconds < 0 {
		return 0, errors.New("seconds cannot be negative in slowecho parameters")
	}
	return time.Duration(seconds) * time.Second, nil

}

// ValidateParameters validates the parameters that will be passed to the Run
// and Resume methods of the test step.
func (e *Step) ValidateParameters(params test.TestStepParameters) error {
	if t := params.GetOne("text"); t.IsEmpty() {
		return errors.New("missing 'text' field in slowecho parameters")
	}
	secStr := params.GetOne("sleep")
	if secStr.IsEmpty() {
		return errors.New("missing 'sleep' field in slowecho parameters")
	}

	// no expression expansion here
	if len(secStr.Raw()) != 1 {
		return fmt.Errorf("invalid empty 'sleep' parameter: %v", secStr)
	}
	_, err := sleepTime(secStr.Raw())
	if err != nil {
		return err
	}
	return nil
}

// Run executes the step
func (e *Step) Run(cancel, pause <-chan struct{}, ch test.TestStepChannels, params test.TestStepParameters, ev testevent.Emitter) error {
	sleep, err := sleepTime(params.GetOne("sleep").String())
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
processing:
	for {
		select {
		case t := <-ch.In:
			if t == nil {
				// no more targets incoming
				wg.Wait()
				return nil
			}
			wg.Add(1)
			go func(t *target.Target) {
				defer wg.Done()
				log.Infof("Waiting %v for target %s", sleep, t.Name)
				select {
				case <-cancel:
					log.Infof("Returning because cancellation is requested")
					return
				case <-pause:
					log.Infof("Returning because pause is requested")
					return
				case <-time.After(sleep):
				}
				log.Infof("target %s: %s", t, params.GetOne("text"))
				select {
				case <-cancel:
					log.Debug("Returning because cancellation is requested")
					return
				case <-pause:
					log.Debug("Returning because pause is requested")
					return
				default:
					ch.Out <- t
				}
			}(t)
		case <-cancel:
			log.Infof("Requested cancellation")
			break processing
		case <-pause:
			log.Infof("Requested pause")
			break processing
		}
	}
	log.Debugf("Waiting for all goroutines to terminate")
	wg.Wait()
	log.Debugf("All goroutines terminated")
	return nil
}

// CanResume tells whether this step is able to resume.
func (e Step) CanResume() bool {
	return false
}

// Resume tries to resume a previously interrupted test step. EchoStep cannot
// resume.
func (e Step) Resume(cancel, pause <-chan struct{}, _ test.TestStepChannels, _ test.TestStepParameters, ev testevent.EmitterFetcher) error {
	return &cerrors.ErrResumeNotSupported{StepName: Name}
}

// graceful package shutdown or reboot current process gracefully.
//
// Copyright 2016 HenryLee. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package graceful

import (
	"context"
	"time"
)

// MinShutdownTimeout the default time-out period for the process shutdown.
const MinShutdownTimeout = 5 * time.Second

var (
	shutdownTimeout time.Duration
	preCloseFunc    func() error
	postCloseFunc   func() error
)

// SetShutdown sets the function which is called after the process shutdown,
// and the time-out period for the process shutdown.
// If 0<=timeout<5s, automatically use 'MinShutdownTimeout'(5s).
// If timeout<0, indefinite period.
// 'preCloseFunc' is executed before closing process, but not guaranteed to be completed.
// 'postCloseFunc' is executed after process are closed, but not guaranteed to be completed.
func SetShutdown(timeout time.Duration, preCloseFunc, postCloseFunc func() error) {
	if timeout < 0 {
		shutdownTimeout = 1<<63 - 1
	} else if timeout < MinShutdownTimeout {
		shutdownTimeout = MinShutdownTimeout
	} else {
		shutdownTimeout = timeout
	}
	preCloseFunc = preCloseFunc
	postCloseFunc = postCloseFunc
}

// Shutdown closes all the frame process gracefully.
// Parameter timeout is used to reset time-out period for the process shutdown.
func Shutdown(timeout ...time.Duration) {
	log.Infof("shutting down process...")

	contextExec(timeout, "shutdown", func(ctxTimeout context.Context) <-chan struct{} {
		endCh := make(chan struct{})
		go func() {
			defer close(endCh)

			var graceful = true

			if preCloseFunc != nil {
				if err := preCloseFunc(); err != nil {
					log.Errorf("[shutdown-preClose] %s", err.Error())
					graceful = false
				}
			}

			graceful = shutdown(ctxTimeout, "shutdown") && graceful

			if graceful {
				log.Infof("process are shutted down gracefully!")
			} else {
				log.Infof("process are shutted down, but not gracefully!")
			}
		}()
		return endCh
	})
}

func contextExec(timeout []time.Duration, action string, deferCallback func(ctxTimeout context.Context) <-chan struct{}) {
	if len(timeout) > 0 {
		SetShutdown(timeout[0], preCloseFunc, postCloseFunc)
	}
	ctxTimeout, _ := context.WithTimeout(context.Background(), shutdownTimeout)
	select {
	case <-ctxTimeout.Done():
		if err := ctxTimeout.Err(); err != nil {
			log.Errorf("[%s-timeout] %s", action, err.Error())
		}
	case <-deferCallback(ctxTimeout):
	}
}

func shutdown(ctxTimeout context.Context, action string) bool {
	if postCloseFunc != nil {
		if err := postCloseFunc(); err != nil {
			log.Errorf("[%s-postClose] %s", action, err.Error())
			return false
		}
	}

	return true
}

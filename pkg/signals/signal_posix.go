// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package signals

import (
	"os"
	"syscall"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGINT}

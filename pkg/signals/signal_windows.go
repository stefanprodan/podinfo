// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package signals

import (
	"os"
)

var shutdownSignals = []os.Signal{os.Interrupt}

// Copyright 2023 Stefan Prodan
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"strconv"
	"strings"
)

// SemVer validates the input version string and extracts the major and minor version numbers.
// When Minimum is set, the major and minor parts must be greater or equal to the minimum
// or a validation error is returned.
#SemVer: {
	// Input version string in strict semver format.
	#Version!: string & =~"^\\d+\\.\\d+\\.\\d+(-[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

	// Minimum is the minimum allowed MAJOR.MINOR version.
	#Minimum: *"0.0.0" | string & =~"^\\d+\\.\\d+\\.\\d+(-[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

	let minMajor = strconv.Atoi(strings.Split(#Minimum, ".")[0])
	let minMinor = strconv.Atoi(strings.Split(#Minimum, ".")[1])

	major: int & >=minMajor
	major: strconv.Atoi(strings.Split(#Version, ".")[0])

	minor: int & >=minMinor
	minor: strconv.Atoi(strings.Split(#Version, ".")[1])
}

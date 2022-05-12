package main

import (
	"tool/cli"
	"encoding/yaml"
)

command: gen: {
	task: print: cli.Print & {
		text: yaml.MarshalStream([ for x in objects {x}])
	}
}

package database

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type NewDBConnectionTestMetadata struct {
	shouldErrorOccur bool
	connectionParams *ConnectionInitializationParams
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

// TestNewDbConnection tests connections attempts to database node
func TestNewDbConnection(t *testing.T) {
	ctx := context.Background()
	var scenarios = getNewDbConnectionTestScenarios()

	for _, scenario := range scenarios {
		conn, err := New(ctx, scenario.connectionParams)
		if err != nil && !scenario.shouldErrorOccur {
			t.Errorf("obtained error but not expected - %s", err.Error())
		}

		if err == nil && scenario.shouldErrorOccur {
			t.Errorf("failed to obtain expected error - %s", err.Error())
		}

		if !scenario.shouldErrorOccur {
			assert.NotNil(t, conn)
		}
	}
}

// getNewDbConnectionTestScenarios returns a set of test scenarios to test creation of new database connections
func getNewDbConnectionTestScenarios() []NewDBConnectionTestMetadata {
	return []NewDBConnectionTestMetadata{
		{
			shouldErrorOccur: false,
			connectionParams: &DefaultConnInitializationParams,
		},
		{
			shouldErrorOccur: true,
			connectionParams: nil,
		},
	}
}

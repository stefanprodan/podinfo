package database

import (
	"context"
	"log"
	"math/rand"
	"time"
	"unsafe"

	schema "github.com/SimifiniiCTO/simfiny-microservice-template/pkg/gen/proto/service_schema"
	"github.com/newrelic/go-agent/v3/newrelic"
	"go.uber.org/zap"
)

const (
	DefaultMaxConnectionAttempts  int           = 3
	DefaultMaxRetriesPerOperation int           = 3
	DefaultRetryTimeout           time.Duration = 50 * time.Millisecond
	DefaultRetrySleepInterval     time.Duration = 25 * time.Millisecond
)

const (
	EMPTY         = ""
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var (
	Conn                *Db
	Port                int    = 5433
	Host                string = "localhost"
	User                string = "user_service_db"
	Password            string = "user_service_db"
	Dbname              string = "user_service_db"
	DefaultDbConnParams        = ConnectionParameters{
		Host:         Host,
		User:         User,
		Password:     Password,
		DatabaseName: Dbname,
		Port:         Port,
	}

	DefaultConnInitializationParams = ConnectionInitializationParams{
		ConnectionParams:       &DefaultDbConnParams,
		Logger:                 zap.L(),
		MaxConnectionAttempts:  DefaultMaxConnectionAttempts,
		MaxRetriesPerOperation: DefaultMaxRetriesPerOperation,
		RetryTimeOut:           DefaultRetryTimeout,
		RetrySleepInterval:     DefaultRetrySleepInterval,
	}
)

var src = rand.NewSource(time.Now().UnixNano())

// GenerateRandomString generates a random string based on the size specified by the client
func generateRandomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

// setup sets up a database connection to the test db node
func setup() {
	ctx := context.Background()
	telemetry, _ := newrelic.NewApplication(
		newrelic.ConfigAppName("test-service"),
		newrelic.ConfigLicense("62fd721c712d5863a4e75b8f547b7c1ea884NRAL"),
		func(cfg *newrelic.Config) {
			cfg.ErrorCollector.RecordPanics = true
		},
		// Use nrzap to register the logger with the agent:
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigEnabled(false),
	)
	DefaultConnInitializationParams.Telemetry = telemetry
	// setup database connection before tests
	Conn, _ = New(ctx, &DefaultConnInitializationParams)
}

// teardown tears down a connection object to the test db node
func teardown() {
	db, err := Conn.Conn.Engine.DB()
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()
}

// generateRandomId generates a random id over a range
func generateRandomId(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}

// generateRandomizedAccount generates a random account
func generateRandomizedAccount() *schema.UserAccount {
	return &schema.UserAccount{
		Id:    0,
		Email: generateRandomString(10),
	}
}

// generateAcctWithMissingRequiredFields generates an account object with a set of required fields that are missing
func generateAcctWithMissingRequiredFields() *schema.UserAccount {
	testAcctWitMissingFields := generateRandomizedAccount()
	testAcctWitMissingFields.Email = EMPTY
	return testAcctWitMissingFields
}

package metrics

import (
	"fmt"
)

type Metric struct {
	MetricName  string
	Name        string
	Help        string
	Subsystem   Subsystem
	Namespace   Namespace
	ServiceName string
}

type Counter struct {
	Metric
}

type Summary struct {
	Metric
}

type Gauge struct {
	Metric
}

type Namespace string
type Subsystem string

const (
	RequestNamespace       Namespace = "request.namespace"
	DatabaseNamespace      Namespace = "database.namespace"
	DistributedTxNamespace Namespace = "distributed_transaction.namespace"
	ServiceNamespace       Namespace = "service.namespace"
)

const (
	GrpcSubSystem  Subsystem = "grpc.subsystem"
	DbSubSystem    Subsystem = "database.subsystem"
	ErrorSubSystem Subsystem = "error.subsytem"
)

// ServiceMetrics represents the set of metrics defined for the following service numerous interactions
type ServiceMetrics struct {
	// tracks the number of grpc requests partitioned by name and status code
	// used for monitoring and alerting (RED method)
	GrpcRequestCounter *Metric
	// tracks the latency associated with grpc requests partitioned by service name, target name,
	// status code, and latency
	GrpcRequestLatency *Metric
	// tracks the number of and types of errors encountered by the service
	ErrorCounter *Metric
	// tracks the number of db operations performed
	DbOperationCounter *Metric
	// tracks the latency of various db operations
	DbOperationLatency *Metric
}

// NewServiceMetrics instantiates a new service metric object made up of all metrics tied to this service
func NewServiceMetrics(serviceName *string) (*ServiceMetrics, error) {
	if serviceName == nil {
		return nil, fmt.Errorf("invalid input argument. service name cannot be nil")
	}
	return &ServiceMetrics{
		DbOperationCounter: NewDbOperationCounter(*serviceName),
		DbOperationLatency: NewDbOperationLatency(*serviceName),
		ErrorCounter:       NewErrorCounter(*serviceName),
		GrpcRequestCounter: NewGrpcRequestCounter(*serviceName),
		GrpcRequestLatency: NewGrpcRequestLatency(*serviceName),
	}, nil
}

// NewDbOperationCounter instantiates a new metric around tracking the number of db requests made by the service
func NewDbOperationCounter(serviceName string) *Metric {
	return &Metric{
		MetricName:  fmt.Sprintf("%s.db.operation.counter", serviceName),
		ServiceName: serviceName,
		Help:        "Tracks the number of db tx processed by the service",
		Subsystem:   DbSubSystem,
		Namespace:   DatabaseNamespace,
	}
}

// NewGrpcRequestLatency instantiates a new metric object around tracking the latency associated with various gRPC operations
func NewDbOperationLatency(serviceName string) *Metric {
	return &Metric{
		MetricName:  fmt.Sprintf("%s.db.operation.latency", serviceName),
		ServiceName: serviceName,
		Help:        "Tracks the latency of all db tx performed by the service.",
		Subsystem:   DbSubSystem,
		Namespace:   DatabaseNamespace,
	}
}

// NewGrpcRequestCounter instantiates a new metric around tracking the number of grpc requests made by the service
func NewErrorCounter(serviceName string) *Metric {
	return &Metric{
		MetricName:  fmt.Sprintf("%s.grpc.error.counter", serviceName),
		ServiceName: serviceName,
		Help:        "Tracks the number of and types of errors encountered by the service",
		Subsystem:   ErrorSubSystem,
		Namespace:   ServiceNamespace,
	}
}

// NewGrpcRequestCounter instantiates a new metric around tracking the number of grpc requests made by the service
func NewGrpcRequestCounter(serviceName string) *Metric {
	return &Metric{
		MetricName:  fmt.Sprintf("%s.grpc.request.counter", serviceName),
		ServiceName: serviceName,
		Help:        "Tracks the number of grpc requests processed by the service. Partitioned by status code and operation",
		Subsystem:   GrpcSubSystem,
		Namespace:   RequestNamespace,
	}
}

// NewGrpcRequestLatency instantiates a new metric object around tracking the latency associated with various gRPC operations
func NewGrpcRequestLatency(serviceName string) *Metric {
	return &Metric{
		MetricName:  fmt.Sprintf("%s.grpc.request.latency", serviceName),
		ServiceName: serviceName,
		Help:        "Tracks the latency of all outgoing grpc requests initiated by the service. Partitioned by status code and operation",
		Subsystem:   GrpcSubSystem,
		Namespace:   RequestNamespace,
	}
}

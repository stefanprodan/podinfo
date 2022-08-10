package metrics

import (
	"errors"
	"time"

	nr "github.com/SimifiniiCTO/core/core-metrics-newrelic"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
)

type MetricType string

var (
	OperationLatencyMetric MetricType = "service.operation.latency"
	OperationStatusMetric  MetricType = "service.operation.status"
	RpcStatusMetric        MetricType = "service.rpc.status"
)

// MetricsEngine enables this service to emit metrics to new relic
type MetricsEngine struct {
	// Metrics encompasses all metrics defined for the various operations this service is part of
	Metrics *ServiceMetrics
	// ServiceName encompasses the name of the service
	ServiceName *string
	// engine is the utility by which metrics are emitted to new-relic
	Engine                  *nr.ServiceMetricsEngine
	metricsReportingEnabled bool
	logger                  *zap.Logger
}

type ServiceDetails struct {
	ServiceName string
	// The version of the service actively deployed
	Version string
	// The service P.O.
	PointOfContact string
	// A link to documentation around the service's functionality and uses
	DocumentationLink string
	// The environment in which the service is actively running and deployed in
	Environment string
	// licencse key for interactions with the new-relic platform
	NewRelicLicenseKey string
}

// NewTelemetry returns a instance of the metrics engine object in which all defined service metrics are present
func NewMetricsEngine(details *ServiceDetails, logger *zap.Logger, metricsReportingEnabled bool) (*MetricsEngine, error) {
	if details == nil {
		return nil, errors.New("invalid input argument. details must be provided")
	}

	if logger == nil {
		return nil, errors.New("invalid input argument. logger cannot be nil")
	}

	metadata := &nr.ServiceMetadata{
		Name:              details.ServiceName,
		Version:           details.Version,
		PointOfContact:    details.PointOfContact,
		DocumentationLink: details.DocumentationLink,
	}

	metrics, err := NewServiceMetrics(&details.ServiceName)
	if err != nil {
		return nil, err
	}

	if !metricsReportingEnabled {
		return &MetricsEngine{
			Engine:                  nil,
			ServiceName:             &details.ServiceName,
			Metrics:                 metrics,
			metricsReportingEnabled: metricsReportingEnabled,
			logger:                  logger,
		}, err
	}

	engine, err := nr.NewServiceMetricsEngine(&details.NewRelicLicenseKey, metadata)
	if err != nil {
		return nil, err
	}

	return &MetricsEngine{
		Engine:                  engine,
		ServiceName:             &details.ServiceName,
		Metrics:                 metrics,
		metricsReportingEnabled: metricsReportingEnabled,
		logger:                  logger,
	}, nil
}

func (me *MetricsEngine) RecordStandardMetrics(op string, isOperationSuccessful bool) {
	me.RecordCounterMetric(me.Metrics.GrpcRequestCounter, op)
}

func (me *MetricsEngine) RecordStandardDtxMetrics(op, dest string, status code.Code, start time.Time) {
	me.RecordLatencyMetric(me.Metrics.GrpcRequestCounter, op, dest, codes.Code(status), time.Since(start))
}

func (me *MetricsEngine) RecordErrorMetric(m *Metric, op, msg string, timeOfOccurence time.Time) {
	if !me.metricsReportingEnabled {
		return
	}

	if !me.validateMetricEngine() {
		me.logger.Panic("unable to emit metrics due to misconfiguration")
	}

	mHandle := me.Engine.Havester.MetricAggregator()
	mHandle.Summary(m.MetricName, map[string]interface{}{
		"service.source":    m.ServiceName,
		"service.operation": op,
		"error.message":     msg,
		"metric.help":       m.Help,
		"metric.namespace":  m.Namespace,
		"metric.subsystem":  m.Subsystem,
		"metric.occurence":  timeOfOccurence.String(),
	})
}

func (me *MetricsEngine) RecordLatencyMetric(m *Metric, op, dest string, status codes.Code, start time.Duration) {
	if !me.metricsReportingEnabled {
		return
	}

	if !me.validateMetricEngine() {
		me.logger.Panic("unable to emit metrics due to misconfiguration")
	}

	mHandle := me.Engine.Havester.MetricAggregator()
	mHandle.Summary(m.MetricName, map[string]interface{}{
		"service.source":      m.ServiceName,
		"service.operation":   op,
		"service.destination": dest,
		"metric.help":         m.Help,
		"metric.namespace":    m.Namespace,
		"metric.subsystem":    m.Subsystem,
		"duration":            start,
	})
}

func (me *MetricsEngine) RecordCounterMetric(m *Metric, op string) {
	if !me.metricsReportingEnabled {
		return
	}

	if !me.validateMetricEngine() {
		me.logger.Panic("unable to emit metrics due to misconfiguration")
	}

	mHandle := me.Engine.Havester.MetricAggregator()
	mHandle.Count(m.MetricName, map[string]interface{}{
		"service.source":    m.ServiceName,
		"service.operation": op,
		"metric.help":       m.Help,
		"metric.namespace":  m.Namespace,
		"metric.subsystem":  m.Subsystem,
	})
}

func (me *MetricsEngine) RecordGaugeMetric(m *Metric, op string) {
	if !me.metricsReportingEnabled {
		return
	}

	if !me.validateMetricEngine() {
		me.logger.Panic("unable to emit metrics due to misconfiguration")
	}

	mHandle := me.Engine.Havester.MetricAggregator()
	mHandle.Gauge(m.MetricName, map[string]interface{}{
		"grpc.source":    m.ServiceName,
		"grpc.operation": op,
		"grpc.help":      m.Help,
		"grpc.namespace": m.Namespace,
		"grpc.subsystem": m.Subsystem,
	})
}

func (me *MetricsEngine) validateMetricEngine() bool {
	if me == nil || (me.Engine == nil && !me.metricsReportingEnabled) || me.Engine.Havester == nil {
		return false
	}

	return true
}

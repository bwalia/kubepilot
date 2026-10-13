package otelautopilot

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// maxOTLPBody caps one decoded OTLP/HTTP request. The collector batches, so
// a single request is rarely more than a few MiB.
const maxOTLPBody = 32 << 20

// keptSpanAttributes are the span attributes worth keeping in the managed
// store: enough to draw the service graph, name virtual nodes and explain an
// error, without holding every attribute an instrumentation library emits.
var keptSpanAttributes = map[string]bool{
	"peer.service": true, "server.address": true, "server.port": true,
	"net.peer.name": true, "http.host": true, "url.full": true, "http.url": true,
	"http.route": true, "url.path": true, "http.target": true,
	"http.request.method": true, "http.method": true,
	"http.response.status_code": true, "http.status_code": true,
	"rpc.system": true, "rpc.service": true, "rpc.method": true,
	"db.system": true, "db.system.name": true, "db.name": true, "db.namespace": true,
	"db.operation": true, "db.operation.name": true,
	"messaging.system": true, "messaging.destination.name": true,
	"error.type": true, "exception.type": true, "exception.message": true,
}

// keptResourceAttributes identify where a span ran.
var keptResourceAttributes = map[string]bool{
	"service.name": true, "service.namespace": true, "service.version": true,
	"k8s.namespace.name": true, "k8s.pod.name": true, "k8s.node.name": true,
	"k8s.deployment.name": true, "k8s.statefulset.name": true, "k8s.daemonset.name": true,
	"k8s.container.name": true, "deployment.environment": true, "deployment.environment.name": true,
	"telemetry.sdk.language": true,
}

// ReadOTLPBody returns the request body, gunzipped when the collector
// compressed it (the otlphttp exporter's default).
func ReadOTLPBody(r *http.Request) ([]byte, error) {
	var reader io.Reader = io.LimitReader(r.Body, maxOTLPBody)
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("invalid gzip body: %w", err)
		}
		defer gz.Close()
		reader = io.LimitReader(gz, maxOTLPBody)
	}
	return io.ReadAll(reader)
}

// IsOTLPJSON reports whether the request carries OTLP/JSON rather than protobuf.
func IsOTLPJSON(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

// IngestOTLPTraces decodes an ExportTraceServiceRequest into the store and
// returns the number of spans accepted.
func (s *Store) IngestOTLPTraces(body []byte, isJSON bool) (int, error) {
	req := &coltrace.ExportTraceServiceRequest{}
	if err := unmarshalOTLP(body, isJSON, req); err != nil {
		return 0, err
	}
	var spans []SpanRecord
	for _, rs := range req.GetResourceSpans() {
		res := attrMap(rs.GetResource().GetAttributes(), keptResourceAttributes)
		service := res["service.name"]
		for _, ss := range rs.GetScopeSpans() {
			for _, sp := range ss.GetSpans() {
				spans = append(spans, spanRecord(sp, service, res))
			}
		}
	}
	if len(spans) > 0 {
		s.IngestSpans(spans...)
	}
	return len(spans), nil
}

func spanRecord(sp *tracepb.Span, service string, res map[string]string) SpanRecord {
	labels := make(map[string]string, len(res)+8)
	for k, v := range res {
		labels[k] = v
	}
	for k, v := range attrMap(sp.GetAttributes(), keptSpanAttributes) {
		labels[k] = v
	}
	statusMsg := sp.GetStatus().GetMessage()
	for _, ev := range sp.GetEvents() {
		if ev.GetName() != "exception" {
			continue
		}
		ex := attrMap(ev.GetAttributes(), map[string]bool{"exception.type": true, "exception.message": true})
		if ex["exception.type"] != "" {
			labels["exception.type"] = ex["exception.type"]
		}
		if ex["exception.message"] != "" {
			labels["exception.message"] = ex["exception.message"]
			if statusMsg == "" {
				statusMsg = ex["exception.message"]
			}
		}
	}

	status := "UNSET"
	switch sp.GetStatus().GetCode() {
	case tracepb.Status_STATUS_CODE_OK:
		status = "OK"
	case tracepb.Status_STATUS_CODE_ERROR:
		status = "ERROR"
	}

	start := time.Unix(0, int64(sp.GetStartTimeUnixNano())).UTC()
	end := time.Unix(0, int64(sp.GetEndTimeUnixNano())).UTC()
	dur := end.Sub(start)
	if dur < 0 {
		dur = 0
	}
	plane := PlaneApp
	if res["k8s.namespace.name"] == "kube-system" {
		plane = PlaneCluster
	}
	return SpanRecord{
		TraceID:       hex.EncodeToString(sp.GetTraceId()),
		SpanID:        hex.EncodeToString(sp.GetSpanId()),
		ParentID:      hex.EncodeToString(sp.GetParentSpanId()),
		Name:          sp.GetName(),
		Service:       service,
		Status:        status,
		StatusMessage: truncate(statusMsg, 300),
		Kind:          spanKind(sp.GetKind()),
		Duration:      dur,
		Labels:        labels,
		Plane:         plane,
		Timestamp:     start,
	}
}

func spanKind(k tracepb.Span_SpanKind) string {
	switch k {
	case tracepb.Span_SPAN_KIND_SERVER:
		return "server"
	case tracepb.Span_SPAN_KIND_CLIENT:
		return "client"
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return "producer"
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return "consumer"
	default:
		return "internal"
	}
}

// IngestOTLPLogs decodes an ExportLogsServiceRequest into the store.
func (s *Store) IngestOTLPLogs(body []byte, isJSON bool) (int, error) {
	req := &collogs.ExportLogsServiceRequest{}
	if err := unmarshalOTLP(body, isJSON, req); err != nil {
		return 0, err
	}
	var records []LogRecord
	for _, rl := range req.GetResourceLogs() {
		res := attrMap(rl.GetResource().GetAttributes(), keptResourceAttributes)
		for _, sl := range rl.GetScopeLogs() {
			for _, lr := range sl.GetLogRecords() {
				labels := make(map[string]string, len(res)+2)
				for k, v := range res {
					labels[k] = v
				}
				if id := hex.EncodeToString(lr.GetTraceId()); id != "" {
					labels["trace_id"] = id
				}
				if id := hex.EncodeToString(lr.GetSpanId()); id != "" {
					labels["span_id"] = id
				}
				ts := lr.GetTimeUnixNano()
				if ts == 0 {
					ts = lr.GetObservedTimeUnixNano()
				}
				sev := lr.GetSeverityText()
				if sev == "" && lr.GetSeverityNumber() != 0 {
					sev = strings.TrimPrefix(lr.GetSeverityNumber().String(), "SEVERITY_NUMBER_")
				}
				records = append(records, LogRecord{
					Timestamp: time.Unix(0, int64(ts)).UTC(),
					Severity:  sev,
					Body:      truncate(anyValueString(lr.GetBody()), 2000),
					Labels:    labels,
					Plane:     PlaneApp,
					Service:   res["service.name"],
				})
			}
		}
	}
	if len(records) > 0 {
		s.IngestLogs(records...)
	}
	return len(records), nil
}

// IngestOTLPMetrics decodes an ExportMetricsServiceRequest, keeping the
// latest value of each gauge/sum data point and the mean of histograms.
func (s *Store) IngestOTLPMetrics(body []byte, isJSON bool) (int, error) {
	req := &colmetrics.ExportMetricsServiceRequest{}
	if err := unmarshalOTLP(body, isJSON, req); err != nil {
		return 0, err
	}
	var samples []MetricSample
	add := func(m *metricspb.Metric, service string, res map[string]string, attrs []*commonpb.KeyValue, ts uint64, v float64) {
		labels := make(map[string]string, len(res)+len(attrs))
		for k, val := range res {
			labels[k] = val
		}
		for k, val := range attrMap(attrs, nil) {
			labels[k] = val
		}
		samples = append(samples, MetricSample{
			Name: m.GetName(), Value: v, Unit: m.GetUnit(), Labels: labels,
			Plane: PlaneApp, Service: service, Timestamp: time.Unix(0, int64(ts)).UTC(),
		})
	}
	for _, rm := range req.GetResourceMetrics() {
		res := attrMap(rm.GetResource().GetAttributes(), keptResourceAttributes)
		service := res["service.name"]
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				switch d := m.GetData().(type) {
				case *metricspb.Metric_Gauge:
					for _, dp := range d.Gauge.GetDataPoints() {
						add(m, service, res, dp.GetAttributes(), dp.GetTimeUnixNano(), numberValue(dp))
					}
				case *metricspb.Metric_Sum:
					for _, dp := range d.Sum.GetDataPoints() {
						add(m, service, res, dp.GetAttributes(), dp.GetTimeUnixNano(), numberValue(dp))
					}
				case *metricspb.Metric_Histogram:
					for _, dp := range d.Histogram.GetDataPoints() {
						if dp.GetCount() > 0 {
							add(m, service, res, dp.GetAttributes(), dp.GetTimeUnixNano(), dp.GetSum()/float64(dp.GetCount()))
						}
					}
				}
			}
		}
	}
	if len(samples) > 0 {
		s.IngestMetrics(samples...)
	}
	return len(samples), nil
}

func numberValue(dp *metricspb.NumberDataPoint) float64 {
	if v, ok := dp.GetValue().(*metricspb.NumberDataPoint_AsInt); ok {
		return float64(v.AsInt)
	}
	return dp.GetAsDouble()
}

func unmarshalOTLP(body []byte, isJSON bool, msg proto.Message) error {
	if !isJSON {
		if err := proto.Unmarshal(body, msg); err != nil {
			return fmt.Errorf("invalid OTLP protobuf: %w", err)
		}
		return nil
	}
	fixed, err := hexIDsToBase64(body)
	if err != nil {
		return fmt.Errorf("invalid OTLP JSON: %w", err)
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(fixed, msg); err != nil {
		return fmt.Errorf("invalid OTLP JSON: %w", err)
	}
	return nil
}

// hexIDsToBase64 rewrites traceId/spanId/parentSpanId from the hex encoding
// OTLP/JSON mandates to the base64 that protojson expects for bytes fields.
func hexIDsToBase64(body []byte) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if s, ok := val.(string); ok && (k == "traceId" || k == "spanId" || k == "parentSpanId") {
					if raw, err := hex.DecodeString(s); err == nil {
						t[k] = base64.StdEncoding.EncodeToString(raw)
					}
					continue
				}
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(doc)
	return json.Marshal(doc)
}

// attrMap flattens OTLP attributes to strings, keeping only keys in keep
// (all keys when keep is nil).
func attrMap(attrs []*commonpb.KeyValue, keep map[string]bool) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, kv := range attrs {
		if keep != nil && !keep[kv.GetKey()] {
			continue
		}
		if v := anyValueString(kv.GetValue()); v != "" {
			out[kv.GetKey()] = v
		}
	}
	return out
}

func anyValueString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch t := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return t.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(t.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(t.DoubleValue, 'f', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(t.BoolValue)
	case *commonpb.AnyValue_ArrayValue:
		parts := make([]string, 0, len(t.ArrayValue.GetValues()))
		for _, item := range t.ArrayValue.GetValues() {
			parts = append(parts, anyValueString(item))
		}
		return strings.Join(parts, ",")
	case *commonpb.AnyValue_KvlistValue:
		parts := make([]string, 0, len(t.KvlistValue.GetValues()))
		for _, kv := range t.KvlistValue.GetValues() {
			parts = append(parts, kv.GetKey()+"="+anyValueString(kv.GetValue()))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

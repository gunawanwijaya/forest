package sdk

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	metric_aggregator_histogram "go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	metric_controller_basic "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	metric_processor_basic "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	metric_selector_simple "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	sdk_resource "go.opentelemetry.io/otel/sdk/resource"
	sdk_trace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type open_telemetry struct{}

// nolint: gochecknoglobals
var (
	OTel  open_telemetry
	logID = strconv.FormatInt(time.Now().UnixMicro(), 36)
	sOUT  = os.Stdout
	sERR  = os.Stderr
)

// -----------------------------------------------------------------------------
// Tracer
// -----------------------------------------------------------------------------

type TracerConfiguration struct {
	Name   string
	Jaeger struct{ URL string }
	OTLP   struct {
		GRPC struct{ URL string }
		HTTP struct{ URL string }
	}
}

func (open_telemetry) NewTracer(ctx context.Context, c *TracerConfiguration) *Tracer {
	if t, ok := ctx.Value(tracerCtxKey{}).(*Tracer); ok && t != nil {
		t.tracerProviderWrap.sdk = nil

		return t
	}

	var spanExporter sdk_trace.SpanExporter

	if c == nil {
		return nil
	} else if c.OTLP.GRPC.URL != "" {
		spanExporter = otlptrace.NewUnstarted(
			otlptracegrpc.NewClient(
				otlptracegrpc.WithInsecure(),
				otlptracegrpc.WithEndpoint(c.OTLP.GRPC.URL),
			),
		)
	} else if c.OTLP.HTTP.URL != "" {
		spanExporter = otlptrace.NewUnstarted(
			otlptracehttp.NewClient(
				otlptracehttp.WithInsecure(),
				otlptracehttp.WithEndpoint(c.OTLP.HTTP.URL),
				otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
			),
		)
	} else if c.Jaeger.URL != "" {
		var err error

		spanExporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(c.Jaeger.URL)))
		if err != nil {
			return nil
		}
	} else {
		return nil
	}

	resource := sdk_resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.TelemetrySDKLanguageGo,
		semconv.HostArchKey.String(runtime.GOARCH),
		semconv.ServiceNameKey.String(c.Name),
		semconv.NetHostNameKey.String(func() string {
			hostname, _ := os.Hostname()

			return hostname
		}()),
		semconv.NetHostIPKey.StringSlice(func() []string {
			ipList := make([]string, 0)
			addrs, _ := net.InterfaceAddrs()

			for _, addr := range addrs {
				ipnet, ok := addr.(*net.IPNet)
				if ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					ipList = append(ipList, ipnet.IP.String())
				}
			}

			return ipList
		}()),
		semconv.OSNameKey.String(runtime.GOOS),
	)

	tp := sdk_trace.NewTracerProvider(
		sdk_trace.WithBatcher(spanExporter),
		sdk_trace.WithResource(resource),
	)

	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tp)

	return &Tracer{tp.Tracer(c.Name), tracerProviderWrap{tp}}
}

type tracerCtxKey struct{}

type tracerProviderWrap struct{ sdk *sdk_trace.TracerProvider }

func (x tracerProviderWrap) RegisterSpanProcessor(s sdk_trace.SpanProcessor) {
	if x.sdk != nil {
		x.sdk.RegisterSpanProcessor(s)
	}
}

func (x tracerProviderWrap) UnregisterSpanProcessor(s sdk_trace.SpanProcessor) {
	if x.sdk != nil {
		x.sdk.UnregisterSpanProcessor(s)
	}
}

func (x tracerProviderWrap) ForceFlush(ctx context.Context) error {
	if x.sdk != nil {
		return x.sdk.ForceFlush(ctx)
	}

	return nil
}

func (x tracerProviderWrap) Shutdown(ctx context.Context) error {
	if x.sdk != nil {
		return x.sdk.Shutdown(ctx)
	}

	return nil
}

type Tracer struct {
	trace.Tracer
	tracerProviderWrap
}

func (t *Tracer) WithContext(ctx context.Context) context.Context {
	if t_, ok := ctx.Value(tracerCtxKey{}).(*Tracer); ok {
		if t == t_ {
			return ctx
		}
	}

	return context.WithValue(ctx, tracerCtxKey{}, t)
}

// -----------------------------------------------------------------------------
// Meter
// -----------------------------------------------------------------------------

type MeterConfiguration struct {
	Name       string
	Prometheus struct {
		HTTPHandlerCallback func(http.Handler)
	}
}

func (open_telemetry) NewMeter(ctx context.Context, c *MeterConfiguration) *Meter {
	if m, ok := ctx.Value(meterCtxKey{}).(*Meter); ok && m != nil {
		return m
	}

	config := prometheus.Config{}
	ctrl := metric_controller_basic.New(
		metric_processor_basic.NewFactory(
			metric_selector_simple.NewWithHistogramDistribution(
				metric_aggregator_histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			metric_processor_basic.WithMemory(true),
		),
	)

	exporter, err := prometheus.New(config, ctrl)
	if err != nil {
		return nil
	}

	if c == nil {
		return nil
	} else if c.Prometheus.HTTPHandlerCallback != nil {
		c.Prometheus.HTTPHandlerCallback(exporter)
	}

	mp := exporter.MeterProvider()

	global.SetMeterProvider(mp)

	return &Meter{mp.Meter(c.Name), nil}
}

type meterCtxKey struct{}

type Meter struct {
	metric.Meter
	_ interface{}
}

func (m *Meter) WithContext(ctx context.Context) context.Context {
	if m_, ok := ctx.Value(meterCtxKey{}).(*Meter); ok {
		if m == m_ {
			return ctx
		}
	}

	return context.WithValue(ctx, meterCtxKey{}, m)
}

func (m *Meter) Must() metric.MeterMust { return metric.Must(m.Meter) }

// -----------------------------------------------------------------------------
// Logger
// -----------------------------------------------------------------------------

func (open_telemetry) NewLogger(ctx context.Context, c ...io.Writer) *Logger {
	ws, z := make([]io.Writer, 0), new(zerolog.Logger)
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		//
	})

	for j := 0; j < len(c); j++ {
		if c[j] != nil {
			ws = append(ws, c[j])
		}
	}

	if len(c) > 0 {
		switch len(ws) {
		case 0:
			*z = zerolog.Nop()
		case 1:
			*z = zerolog.New(ws[0]).Hook(hook)
		default:
			*z = zerolog.New(zerolog.MultiLevelWriter(ws...)).Hook(hook)
		}
	} else if zz := zerolog.Ctx(ctx); zz != nil {
		*z = *zz
	}

	*z = z.With().Timestamp().Logger()

	dir := os.TempDir()
	tempOUT, _ := os.Create(dir + "/temp-" + logID + "-out.log")
	tempERR, _ := os.Create(dir + "/temp-" + logID + "-err.log")

	return &Logger{log.New(z, "", 0), z, tempOUT, tempERR}
}

type loggerCtxKey struct{}

type Logger struct {
	standard *log.Logger
	zerolog  *zerolog.Logger

	tempOUT, tempERR *os.File
}

func (l *Logger) WithContext(ctx context.Context) context.Context {
	if l_, ok := ctx.Value(loggerCtxKey{}).(*Logger); ok {
		if l == l_ {
			return ctx
		}
	}

	return context.WithValue(ctx, loggerCtxKey{}, l)
}

func (l *Logger) Swap() (readOUT, readERR func() ([]byte, error)) {
	os.Stdout, os.Stderr = l.tempOUT, l.tempERR

	log.SetOutput(os.Stderr)

	return func() ([]byte, error) {
			p, err := ioutil.ReadFile(l.tempOUT.Name())
			_ = l.tempOUT.Truncate(0)

			return p, err
		}, func() ([]byte, error) {
			p, err := ioutil.ReadFile(l.tempERR.Name())
			_ = l.tempERR.Truncate(0)

			return p, err
		}
}

func (l *Logger) Unswap() {
	os.Stdout, os.Stderr = sOUT, sERR

	log.SetOutput(os.Stderr)
}

func (l *Logger) S() *log.Logger     { return l.standard }
func (l *Logger) Z() *zerolog.Logger { return l.zerolog }
func (l *Logger) Level(level string) *Logger {
	lv, err := zerolog.ParseLevel(strings.ToLower(level))
	if err == nil {
		*l.zerolog = l.zerolog.Level(lv)
		l.standard.SetOutput(l.zerolog)
	}

	return l
}

func (open_telemetry) NewConsoleWriter(w io.Writer) *zerolog.ConsoleWriter {
	return &zerolog.ConsoleWriter{Out: w}
}
